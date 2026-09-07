# Deployment guide

For hardened single-host and Kubernetes production procedures, release gates,
and rollback, see [production deployment](production-deployment.md). For backup
and recovery objectives, see [disaster recovery](disaster-recovery.md).

## Local Compose deployment

```powershell
Copy-Item .env.example .env
# Set JWT_SECRET to at least 32 cryptographically random characters.
docker compose --env-file .env -f docker/compose.yaml up --build -d
docker compose --env-file .env -f docker/compose.yaml ps
```

The Compose stack exposes the web application on port 3000 and the API on port
8080. PostgreSQL and Redis ports are exposed for local diagnostics; production
deployments should keep them on private networks.

On first start, open `/install`. Compose supplies database and cache settings;
the initializer performs read-only health checks and only collects the first
administrator and company profile. Completion is stored in PostgreSQL with a
secondary lock in `nexaflow_data`. See `installation.md` for recovery rules.

## Production requirements

- Replace all example credentials and use a secret manager or Docker secrets.
- Set `JWT_SECRET` to a random value of at least 32 characters. Production
  startup rejects the development signing secret.
- Terminate TLS at a trusted reverse proxy or load balancer.
- Set `CORS_ALLOWED_ORIGINS` to exact HTTPS application origins.
- Set `NEXT_PUBLIC_API_URL` during the frontend image build.
- Use the bundled pgvector PostgreSQL 18 image or install the `vector` extension
  in an external PostgreSQL service before migrations run.
- Run the backend with a dedicated database owner configured as `NOSUPERUSER`
  and `NOBYPASSRLS`. PostgreSQL superusers always bypass row-level security and are
  rejected by startup migration. For external PostgreSQL, an operator must
  pre-create the database, `vector` extension and application role; do not give
  the backend bootstrap-administrator credentials. Compose runs a one-shot
  `database-role-init` service so both new and existing `postgres_data` volumes
  receive the non-bypass application role before the backend starts.
- Set `AI_API_KEY` to enable ingestion and chat. `AI_BASE_URL`, `AI_CHAT_MODEL`,
  and `AI_EMBEDDING_MODEL` support compatible providers; keep the 1536-dimension
  embedding contract unless a migration changes it.
- Set `OBSERVABILITY_ENABLED=true` and send `OBSERVABILITY_ENDPOINT` to a
  private OpenTelemetry Collector. Configure model token prices if estimated AI
  cost metrics are required. See [observability and reliability](observability.md).
- Size `JOBS_CONCURRENCY`, `JOBS_LEASE_TIMEOUT`, retry bounds and the
  idempotency TTL against the slowest handler. Redis persistence is mandatory:
  durable-job keys are not disposable cache data.
- Configure `KNOWLEDGE_WORKER_INTERVAL`, `KNOWLEDGE_BATCH_SIZE`, and
  `KNOWLEDGE_MAX_ATTEMPTS` for asynchronous knowledge ingestion. Uploads are
  durable only when the configured object store is durable and shared by all
  backend replicas.
- Configure `NOTIFICATIONS_SMTP_*` and `NOTIFICATIONS_WEBHOOK_ALLOWLIST` before
  enabling outbound email or webhook workflow nodes. Webhook hosts must be
  explicitly allowlisted and resolve to public addresses; leave the allowlist
  empty to disable webhooks.
- Set `NOTIFICATIONS_WORKER_INTERVAL`, `NOTIFICATIONS_BATCH_SIZE`, and
  `NOTIFICATIONS_MAX_ATTEMPTS` to match the expected delivery volume and retry
  policy. Notification and knowledge batches run as idempotently scheduled
  Redis jobs only after installation is complete.
- Use managed or backed-up PostgreSQL with point-in-time recovery.
- Enable Redis authentication, persistence, memory limits, and eviction policy
  appropriate to each cache/job workload.
- Pin images by immutable digest in a release manifest.
- Export correlated logs, OTLP traces and metrics to an external observability
  system; install the supplied SLO alerts and dashboard as a starting point.
- Keep the installation data directory private and back it up with the database.
- Run release migrations as a separate deployment step before sending traffic
  to a new version. The runner validates immutable checksums, holds a PostgreSQL
  advisory lock, and commits one migration plus ledger row per transaction.
  Follow [migration operations](migrations.md) and the
  [release/rollback policy](release-policy.md).

## Container behavior

Both application images use multi-stage builds and run as non-root users. The
Next.js image uses standalone output. The pgvector image is based on PostgreSQL
18, and its data is mounted at
`/var/lib/postgresql`, matching the official image's version-specific data
layout introduced in PostgreSQL 18.

## Backup boundary

The named volumes in Compose are for developer convenience, not a backup.
Production recovery must be tested against encrypted database backups and
protected object storage before a release is considered deployable. The release
preflight requires fresh, signed restore evidence.
