# Production deployment

NexaFlow provides two production references. `docker/compose.production.yaml`
is a hardened single-host deployment for small installations. The Kubernetes
reference in `deploy/kubernetes` is the high-availability deployment and keeps
PostgreSQL, Redis, object storage and OpenTelemetry outside the cluster
workloads as managed services.

## Release gates

A production promotion requires all of the following:

- backend, frontend and backup images pinned with `@sha256` digests;
- a signed restore-drill result completed within the last eight days;
- the currently deployed `/health/ready` endpoint passing;
- server-side Kubernetes dry-run passing before apply;
- both Deployments completing their zero-unavailable rollout.

`sh scripts/release-preflight.sh` enforces the first three gates. It requires
`PUBLIC_URL`, `BACKEND_IMAGE`, `FRONTEND_IMAGE`, `BACKUP_IMAGE`,
`RESTORE_EVIDENCE_FILE`, `RESTORE_EVIDENCE_SIGNATURE` and
`BACKUP_SIGNING_PUBLIC_KEY_FILE`. `sh scripts/deploy-kubernetes.sh` then renders
the immutable images into a temporary Kustomize tree, performs a server-side
dry-run, applies it and waits for rollout and readiness.

Only the first deployment may have no current readiness endpoint. In that case,
set `INITIAL_DEPLOY_CONFIRM=initial-deploy`; this does not bypass immutable
images, restore evidence, signature verification or Kubernetes validation.

## Single-host Compose

1. Copy `.env.production.example` to `.env.production.local` and replace every
   placeholder, including every image digest.
2. Create the files listed in `secrets/README.md.example` with mode `0600`.
3. Configure object retention as described in `disaster-recovery.md`.
4. Validate and start the stack:

```sh
docker compose --env-file .env.production.local -f docker/compose.production.yaml config
docker compose --env-file .env.production.local -f docker/compose.production.yaml up -d
docker compose --env-file .env.production.local -f docker/compose.production.yaml ps
```

Only the unprivileged Nginx proxy publishes ports. PostgreSQL and Redis remain
on an internal network. Application filesystems are read-only, Linux
capabilities are dropped, resource/PID/log limits are set, and services use
health dependencies and graceful stop periods. TLS certificates are mounted as
secrets. This topology is recoverable but is not highly available: a host
failure causes downtime until that host or a replacement is restored.

Install the units in `deploy/systemd` to schedule the daily backup and weekly
restore drill. Review their `/opt/nexaflow` working directory before enabling
the timers. The `backup` service cannot read the age identity or database
administrator password; only the separately invoked `recovery` service receives
those secrets.

## Kubernetes

Follow `deploy/kubernetes/README.md`. Supply the two Secrets through External
Secrets, Sealed Secrets or a cloud secret-store CSI driver; never add the
example Secret to Kustomize. Secret values are mounted as files rather than
copied into the backend process environment.

The reference uses two replicas, rolling updates with `maxUnavailable: 0`,
startup/liveness/readiness probes, graceful pre-stop delay, Pod Security
restricted settings, topology spreading, disruption budgets, autoscaling and
default-deny NetworkPolicies. At least two worker nodes and a disruption-aware
ingress controller are needed for those controls to provide availability.

Customize hostnames, storage endpoints, resource sizing, ingress class and
NetworkPolicy namespace labels before deployment. The backup temporary volume
must exceed the largest expected uncompressed database archive.

## Rollback

Application rollback means re-running deployment with the previous signed
image digests. Do not reverse an expand-compatible database migration. If a
release includes a contract migration, complete the compatibility window and
take a verified backup before contracting. See `release-policy.md` and
`runbooks/production-rollout.md`.
