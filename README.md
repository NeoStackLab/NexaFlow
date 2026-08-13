# NexaFlow

NexaFlow is an open-source, AI-native, multi-tenant business operating system for building internal enterprise applications. It combines dynamic data models, records, forms, workflows, permissions, files, knowledge, and AI assistance behind one tenant-aware platform.

The repository uses a Go API and a Next.js web application. Docker Compose is the recommended way to run the complete stack.

![NexaFlow dashboard](docs/screenshots/showcase/06-admin-dashboard.png)

## What NexaFlow is for

NexaFlow is a configurable foundation for internal business systems. Teams define their own entities, fields, records, forms, approval flows, files, and permissions instead of starting from a fixed industry template. Typical uses include:

- CRM-style customer, account, renewal, and sales operations
- Lightweight ERP workflows for orders, procurement, inventory requests, and approvals
- Project, service, compliance, and document processes that need auditable roles
- Department-specific applications that share one tenant-aware data and workflow core

It is best understood as a low-code business operations platform rather than a finished, industry-specific CRM or full ERP suite. The platform supplies reusable primitives; each workspace shapes the business schema and process rules.

## AI module

The AI workspace is a permission-scoped assistant for tenant knowledge and authorized business data. It is designed to help users find policy or product information, summarize uploaded documents, and answer questions about the current workspace without bypassing RBAC.

The flow is:

1. An administrator uploads PDF, DOCX, XLSX, TXT, Markdown, CSV, or JSON knowledge files.
2. NexaFlow extracts and chunks the content, creates embeddings, and stores vectors in pgvector.
3. A user asks a question in the AI workspace.
4. The server performs tenant-scoped knowledge search and may read bounded business records only when the current user has the relevant record permission.
5. Sources, tool calls, conversations, and token usage are persisted for audit and usage metering.

AI is provider-agnostic through an OpenAI-compatible API. Set AI_API_KEY to enable ingestion and chat; AI_BASE_URL, AI_CHAT_MODEL, and AI_EMBEDDING_MODEL select a compatible provider and models. Without a key, the endpoint fails closed with an unavailable response and never generates a fake answer. See the [AI API contract](docs/api.md#knowledge-and-ai-agent) and [deployment requirements](docs/deployment.md#production-requirements) for details.

## Product showcase

The screenshots below are current English-locale captures from the local application. They use an isolated showcase workspace with demo data and do not contain production credentials.

![Installation welcome](docs/screenshots/showcase/01-install-welcome.png)
![Service readiness](docs/screenshots/showcase/02-service-readiness.png)
![Sign in](docs/screenshots/showcase/03-login.png)
![Entity designer](docs/screenshots/showcase/07-admin-entities.png)
![Workflow center](docs/screenshots/showcase/08-admin-workflows.png)
![AI workspace](docs/screenshots/showcase/09-admin-ai.png)

## Capabilities

- Six-step first-run installer with PostgreSQL, Redis, storage, and capability checks
- Multi-tenant workspaces with JWT sessions, RBAC, tenant-scoped menus, and audit-friendly permissions
- Dynamic entities and fields backed by validated JSONB records
- Generic business-data CRUD, form builder, and generated JSON Schema
- Executable approval, condition, and notification workflows
- Tenant file space with local, S3, and Cloudflare R2 storage providers
- pgvector-backed knowledge search and permission-scoped AI assistance
- SaaS plans, usage metering, Stripe Checkout, and verified webhooks
- Chinese and English UI, with Simplified Chinese as the default locale

## Technology stack

| Layer | Technology |
| --- | --- |
| Web | Next.js 16.3, React 19, TypeScript, Tailwind CSS 4, TanStack Query |
| API | Go 1.26.5, Gin, GORM, Zap |
| Database | PostgreSQL 18 with pgvector 0.8.6 |
| Cache | Redis 8.8 |
| Runtime | Docker Desktop, Docker Compose, multi-stage non-root images |

## Repository layout

~~~text
NexaFlow/
├── backend/
│   ├── cmd/server/             # Go API entry point
│   ├── configs/                # Default API configuration
│   └── internal/
│       ├── api/                # Gin route registration
│       ├── handler/            # HTTP input/output layer
│       ├── middleware/         # Auth, tenant, and shared middleware
│       ├── model/              # Domain and persistence models
│       ├── pkg/                # Database, cache, logging, and infrastructure
│       ├── repository/         # PostgreSQL and Redis access
│       └── service/            # Business rules and transaction boundaries
├── frontend/
│   ├── public/                 # Static assets
│   └── src/
│       ├── app/                # Next.js App Router routes
│       ├── components/         # Admin, form, workflow, and shared UI
│       └── lib/                # API clients, state, and domain types
├── docker/compose.yaml         # Full local stack
├── docs/                       # Architecture, API, database, and deployment docs
├── scripts/verify.ps1          # Backend and frontend quality checks
├── .env.example                # Environment template
└── README.md
~~~

The API dependency direction is intentionally fixed:

~~~text
HTTP request → handler → service → repository → PostgreSQL / Redis
~~~

Dynamic entities do not create one PostgreSQL table per entity. Definitions live in entities and entity_fields; business records live in dynamic_records.values as JSONB and are isolated by tenant_id and entity_id.

## Quick start on Windows

### Prerequisites

- Docker Desktop with Docker Compose
- Git, if you are cloning the repository

Confirm Docker is running before starting:

~~~powershell
docker desktop status
docker version
docker compose version
~~~

If Docker Desktop reports starting, wait until it reports running.

### 1. Create the environment file

~~~powershell
Set-Location F:\spacex\NexaFlow
Copy-Item .env.example .env
~~~

Generate a JWT secret compatible with Windows PowerShell 5.1 and older .NET runtimes:

~~~powershell
$bytes = New-Object byte[] 48
$rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create()
$rng.GetBytes($bytes)
$jwtSecret = [Convert]::ToBase64String($bytes)
$rng.Dispose()
(Get-Content .env) -replace '^JWT_SECRET=.*$', "JWT_SECRET=$jwtSecret" | Set-Content .env
~~~

Do not use the RandomNumberGenerator Fill method on older PowerShell/.NET versions; that static method may not exist.

Edit .env and set strong local values at minimum:

~~~dotenv
POSTGRES_PASSWORD=replace-with-a-strong-local-password
REDIS_PASSWORD=replace-with-a-strong-local-password
JWT_SECRET=the-random-value-generated-above
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
CORS_ALLOWED_ORIGINS=http://localhost:3000
~~~

AI, Stripe, S3, and R2 settings are optional. Leave them empty when those capabilities are not needed.

### 2. Build and start the stack

~~~powershell
docker compose --env-file .env -f docker\compose.yaml up --build -d
docker compose --env-file .env -f docker\compose.yaml ps
~~~

The first run downloads base images and builds the application images. Later starts reuse those images and named volumes; they do not reinstall the stack.

Wait until PostgreSQL, Redis, the API, and the web container are all healthy.

### 3. Finish the browser installer

Open http://localhost:3000. An uninstalled instance redirects to /install. Complete the wizard in order:

1. Welcome
2. Service readiness
3. Optional capabilities
4. Platform administrator
5. Organization profile
6. Completion and redirect to the admin console

The installer stores initialization state in PostgreSQL. Uploaded files and database data are kept in Docker named volumes, so closing Docker Desktop or rebooting Windows does not require another installation.

## Daily operations

Start Docker Desktop when it is closed, then start the services:

~~~powershell
Set-Location F:\spacex\NexaFlow
docker desktop start
docker desktop status
docker compose --env-file .env -f docker\compose.yaml up -d
~~~

If Docker Desktop is already running, only the final docker compose up -d command is needed. Do not add --build for normal starts.

Useful URLs:

| Purpose | URL |
| --- | --- |
| Web root | http://localhost:3000 |
| Installer | http://localhost:3000/install |
| Sign in | http://localhost:3000/login |
| Admin console | http://localhost:3000/admin |
| API liveness | http://localhost:8080/health/live |
| API readiness | http://localhost:8080/health/ready |
| Versioned API health | http://localhost:8080/api/v1/health |

Stop services without deleting data:

~~~powershell
docker compose --env-file .env -f docker\compose.yaml stop
~~~

Remove containers but keep named volumes:

~~~powershell
docker compose --env-file .env -f docker\compose.yaml down
~~~

Avoid docker compose down -v: -v deletes the PostgreSQL, Redis, and NexaFlow data volumes.

## Updating the application

After pulling or changing source code, rebuild the application images:

~~~powershell
Set-Location F:\spacex\NexaFlow
docker compose --env-file .env -f docker\compose.yaml up --build -d
docker compose --env-file .env -f docker\compose.yaml ps
~~~

For frontend-only changes:

~~~powershell
docker compose --env-file .env -f docker\compose.yaml build frontend
docker compose --env-file .env -f docker\compose.yaml up -d --no-deps frontend
~~~

Use Ctrl+F5 if the browser still shows an older bundle.

View service logs with:

~~~powershell
docker compose --env-file .env -f docker\compose.yaml logs -f backend frontend
~~~

## Troubleshooting

### Port 5432 or 6379 is already in use

Check for a local PostgreSQL or Redis service before stopping anything. A common Windows PostgreSQL service name is:

~~~powershell
Get-Service postgresql*
Stop-Service postgresql-x64-18
~~~

The Stop-Service command may require an elevated PowerShell window. Only stop the service when it is the process holding the conflicting port.

### Redis is unhealthy

~~~powershell
docker compose --env-file .env -f docker\compose.yaml logs redis
docker compose --env-file .env -f docker\compose.yaml up -d --force-recreate redis
~~~

### The root opens the wrong page

The root route is state-aware: an uninstalled instance opens /install; an installed instance opens /login. Use Ctrl+F5, or open /login and /admin directly.

## Local development

The native toolchain is Go 1.26.5, Node.js 22, pnpm 11, PostgreSQL 18, and Redis 8.

Backend:

~~~powershell
Set-Location backend
go mod download
go test ./...
go run ./cmd/server
~~~

Frontend (in a second PowerShell window):

~~~powershell
Set-Location frontend
corepack enable
corepack pnpm install --frozen-lockfile
corepack pnpm lint
corepack pnpm typecheck
corepack pnpm dev
~~~

Run the repository quality checks from the project root:

~~~powershell
.\scripts\verify.ps1
~~~

## Production deployment notes

docker/compose.yaml is optimized for a single-host installation and local validation. A production deployment should additionally:

1. Inject unique PostgreSQL, Redis, and JWT secrets through a secret manager or Docker Secrets.
2. Terminate HTTPS at a reverse proxy and expose only the web/API entry points.
3. Set CORS_ALLOWED_ORIGINS to the exact HTTPS frontend origin.
4. Build the frontend with a public HTTPS NEXT_PUBLIC_API_URL.
5. Keep PostgreSQL 5432 and Redis 6379 private.
6. Use pgvector-capable PostgreSQL with tested backups and restore drills.
7. Enable object-storage versioning, or back up the nexaflow_data volume when using local storage.
8. Configure Redis authentication, persistence, memory limits, and eviction policy.
9. Export JSON logs, metrics, and alerts to an external observability system.
10. Run tests, go vet, frontend lint/type checks, and a production build before release.

Named volumes are not backups. Recovery must rely on tested PostgreSQL backups and object-storage versioning.

## Documentation

- [Installation design](docs/installation.md)
- [API reference](docs/api.md)
- [System architecture](docs/architecture.md)
- [Database design](docs/database.md)
- [Development guide](docs/development.md)
- [Deployment guide](docs/deployment.md)
- [Security boundaries](docs/security.md)
- [Plugin extension guide](docs/plugin.md)
- [Screenshot catalog](docs/screenshots/README.md)

## License

NexaFlow is released under the [Apache License 2.0](LICENSE). The license permits use, modification, distribution, and commercial use, provided that the required copyright, license, notice, and patent terms are respected. Apache-2.0 also permits hosted or SaaS offerings based on the project.
