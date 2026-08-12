# Installation guide

NexaFlow includes a six-step first-run initializer at `/install`. Docker Compose
owns infrastructure configuration; the browser never asks for or receives
PostgreSQL or Redis credentials. The home route redirects to initialization
until the database completion marker exists.

## Docker installation

```powershell
Copy-Item .env.example .env
docker compose --env-file .env -f docker/compose.yaml up --build -d
```

Open <http://localhost:3000>. Compose waits for healthy PostgreSQL and Redis
before starting the API. Infrastructure credentials stay in `.env` or the
production secret manager and are injected into the backend container.

## Installation sequence

1. Review the build version, Apache 2.0 license, and source repository.
2. Run read-only checks for PostgreSQL, Redis, environment configuration, and
   the persistent data directory. Required failures block continuation.
3. Review optional AI, Stripe, and local/S3/R2 storage capability status. Missing
   optional providers do not block initialization and secrets are never shown.
4. Create the first administrator. Passwords must contain 12-72 characters and
   are stored only as bcrypt hashes with cost 12.
5. Set the first company name, industry, language, and IANA timezone.
6. Complete initialization. The backend revalidates environment-supplied
   PostgreSQL and Redis, applies idempotent migrations, creates the company,
   tenant, `super_admin`, and role bindings transactionally, writes the database
   completion marker, and creates `.install.lock` as a secondary marker.

## Installation files

The authoritative completion marker is
`system_settings.installation.completed_at` in PostgreSQL. The backend also
writes owner-readable `.install.lock` below `INSTALL_DATA_DIR` (default
`./data`, `/app/data` in Compose) for operational visibility. Repeated requests
return HTTP `409`; no infrastructure credential is persisted by the initializer.

## Recovery

- A failed environment check does not modify the database.
- A failed final installation does not create the lock. Correct the reported
  Compose health or validation error and retry.
- Removing only `.install.lock` does not reopen an initialized database. Reset
  is intentionally a database recovery operation; back up PostgreSQL first.
