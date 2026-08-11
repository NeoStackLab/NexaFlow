# Deployment guide

## Local Compose deployment

```powershell
Copy-Item .env.example .env
docker compose --env-file .env -f docker/compose.yaml up --build -d
docker compose --env-file .env -f docker/compose.yaml ps
```

The Compose stack exposes the web application on port 3000 and the API on port
8080. PostgreSQL and Redis ports are exposed for local diagnostics; production
deployments should keep them on private networks.

## Production requirements

- Replace all example credentials and use a secret manager or Docker secrets.
- Terminate TLS at a trusted reverse proxy or load balancer.
- Set `CORS_ALLOWED_ORIGINS` to exact HTTPS application origins.
- Set `NEXT_PUBLIC_API_URL` during the frontend image build.
- Use managed or backed-up PostgreSQL with point-in-time recovery.
- Enable Redis authentication, persistence, memory limits, and eviction policy
  appropriate to each cache/job workload.
- Pin images by immutable digest in a release manifest.
- Export logs and metrics to an external observability system.
- Run schema migrations as a separate, idempotent release step once migrations
  are introduced in Phase 2.

## Container behavior

Both application images use multi-stage builds and run as non-root users. The
Next.js image uses standalone output. PostgreSQL 18 data is mounted at
`/var/lib/postgresql`, matching the official image's version-specific data
layout introduced in PostgreSQL 18.

## Backup boundary

The named volumes in Compose are for developer convenience, not a backup.
Production recovery must be tested against database backups and object storage
versioning before a release is considered deployable.
