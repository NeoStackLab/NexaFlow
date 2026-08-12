# Deployment guide

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
- Set `AI_API_KEY` to enable ingestion and chat. `AI_BASE_URL`, `AI_CHAT_MODEL`,
  and `AI_EMBEDDING_MODEL` support compatible providers; keep the 1536-dimension
  embedding contract unless a migration changes it.
- Use managed or backed-up PostgreSQL with point-in-time recovery.
- Enable Redis authentication, persistence, memory limits, and eviction policy
  appropriate to each cache/job workload.
- Pin images by immutable digest in a release manifest.
- Export logs and metrics to an external observability system.
- Keep the installation data directory private and back it up with the database.
- Run release migrations as separate, idempotent deployment steps for rolling
  production upgrades.

## Container behavior

Both application images use multi-stage builds and run as non-root users. The
Next.js image uses standalone output. The pgvector image is based on PostgreSQL
18, and its data is mounted at
`/var/lib/postgresql`, matching the official image's version-specific data
layout introduced in PostgreSQL 18.

## Backup boundary

The named volumes in Compose are for developer convenience, not a backup.
Production recovery must be tested against database backups and object storage
versioning before a release is considered deployable.
