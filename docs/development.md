# Development guide

## Toolchain

- Go 1.26.5
- Node.js 22.x
- pnpm 11.x
- PostgreSQL 18.x
- Redis 8.x
- Docker Desktop + Compose (recommended for local infrastructure)

## Backend

```powershell
Set-Location backend
go mod download
go test ./...
go run ./cmd/server
```

Configuration is loaded from `backend/configs/config.yaml`. Any nested setting
can be overridden by an uppercase environment variable where dots become
underscores, for example `DATABASE_HOST` or `SERVER_PORT`.

Health endpoints:

- `/health/live` proves that the process can serve HTTP.
- `/health/ready` proves that PostgreSQL and Redis are reachable.
- `/api/v1/health` exposes the readiness result through the versioned API.

## Frontend

```powershell
Set-Location frontend
pnpm install --frozen-lockfile
pnpm lint
pnpm typecheck
pnpm build
pnpm dev
```

Copy `frontend/.env.example` to `frontend/.env.local` when the API URL differs
from the default. `NEXT_PUBLIC_API_URL` is embedded into the browser bundle at
build time and must be supplied during production image builds.

## Adding backend behavior

1. Define domain or persistence data in `internal/model`.
2. Define the persistence contract and implementation in `internal/repository`.
3. Implement business rules in `internal/service` against repository interfaces.
4. Translate HTTP input and output in `internal/handler`.
5. Register routes in `internal/api`.
6. Add service tests and handler/integration tests proportionate to risk.

Handlers must not import GORM or database drivers.

## Quality gate

Run `scripts/verify.ps1` from the repository root. Docker configuration is
validated automatically when the Docker CLI is available.
