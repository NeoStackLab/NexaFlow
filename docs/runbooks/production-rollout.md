# Production rollout runbook

## Before promotion

- Confirm CI, image signing, provenance and SBOM jobs passed.
- Obtain backend, frontend and backup image digests.
- Download the newest production restore evidence and its minisign signature.
- Verify alerts are quiet and the current `/health/ready` is healthy.
- Confirm the release is expand-compatible with the database schema.

Run `sh scripts/release-preflight.sh`. For Kubernetes, run
`sh scripts/deploy-kubernetes.sh`; it repeats the gate, performs the dry-run and
waits for both rollouts. Watch error rate, latency, queue depth, DLQ growth and
database saturation during the rollout.

## Abort and rollback

Abort if readiness fails, the rollout exceeds ten minutes, SLO burn alerts
fire, or durable jobs accumulate abnormally. Reapply the previous immutable
image digests and wait for readiness. Do not roll back a database migration;
use the forward remediation described in `release-policy.md`.

If the previous image cannot read the expanded schema, stop traffic and follow
the database recovery procedure in `disaster-recovery.md`. Record the release,
digests, timing, metrics and decision in the incident timeline.
