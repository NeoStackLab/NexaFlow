# Changelog

All notable changes to NexaFlow will be documented in this file. The format is
based on Keep a Changelog, and releases follow Semantic Versioning.

## [Unreleased]

### Added

- Phase 1 Go/Gin API foundation with clean dependency flow.
- PostgreSQL and Redis readiness checks.
- Next.js enterprise console foundation and backend status integration.
- Reproducible container and local Compose configuration.
- Architecture, development, deployment, and database baseline documentation.
- OpenTelemetry traces and metrics for HTTP, PostgreSQL/GORM, Redis, object
  storage, AI providers, workflows, quotas, and durable jobs.
- Redis-backed priority jobs with idempotency, delayed retries, leases,
  heartbeats, distributed schedules, and bounded DLQ replay.
- SLOs, Prometheus alerts, a Grafana dashboard, and durable-job runbooks.
- Hardened production Compose and Kubernetes references with TLS termination,
  restricted workloads, resource controls, graceful rollouts, PDBs and HPAs.
- Age-encrypted PostgreSQL backups, signed manifests, immutable object keys,
  scheduled isolated restore drills and signed recovery evidence release gates.
