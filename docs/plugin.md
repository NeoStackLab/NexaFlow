# Extension and plugin guide

NexaFlow extensions use stable service boundaries instead of loading arbitrary
code into the API process. This preserves tenant isolation and keeps private
deployments auditable.

## Supported extension surfaces

- REST integrations authenticate as a scoped NexaFlow user and operate through
  permission-checked `/api/v1` endpoints.
- Workflow notification nodes publish durable email and Webhook outbox records.
- AI providers implement the OpenAI-compatible chat and embedding contract.
- Object storage implements the internal `ObjectStore` interface; the bundled
  adapters cover local disk, Amazon S3, and Cloudflare R2.
- Frontend modules add an App Router page and a menu entry guarded by a named
  permission.

## Backend module contract

New domain modules follow `Handler -> Service -> Repository -> Database`.
Handlers own HTTP translation, services own validation and authorization-aware
business rules, and repositories own parameterized persistence. Every
tenant-owned query must bind `tenant_id`; plugins must never accept arbitrary
SQL fragments from a browser or AI tool.

## Permission and migration rules

Define narrowly scoped permissions such as `inventory.view` and
`inventory.manage`. Add schema through idempotent migrations and grant a newly
created permission only during its upgrade migration; startup must not restore
a permission that a tenant administrator revoked.

## Compatibility

External integrations should depend on the versioned REST contract and unified
response envelope. In-process Go interfaces are source-level extension points
and may change between minor releases until a dedicated plugin SDK is versioned.
Secrets belong in environment variables or a secret manager and must never be
stored in plugin manifests, returned by readiness endpoints, or logged.
