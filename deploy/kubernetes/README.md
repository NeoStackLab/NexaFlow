# Kubernetes production reference

This directory deploys only stateless NexaFlow workloads. Use managed,
multi-zone PostgreSQL, Redis and S3-compatible storage with private networking;
do not turn the placeholder hosts or image references into production by
accident.

1. Copy `secret.example.yaml` into a secret-management system. Do not add the
   example to `kustomization.yaml`.
2. Replace every host, origin, bucket and ingress hostname in the ConfigMaps.
3. Set backend, frontend and backup images to signed immutable digests:
   `kustomize edit set image old=image@sha256:<digest>`.
4. Confirm the ingress namespace label used by the NetworkPolicies, then run
   `kubectl apply --server-side --dry-run=server -k deploy/kubernetes`.
5. Apply and wait using `scripts/deploy-kubernetes.sh`.

The Deployments use two minimum replicas, zero-unavailable rolling updates,
startup/liveness/readiness probes, graceful pre-stop delays, restricted pod
security, resource requests/limits, topology spreading, PDBs and HPAs. The
daily encrypted backup and weekly restore drill are separate CronJobs. Adjust
their temporary storage limits to exceed the largest expected uncompressed
database archive.
