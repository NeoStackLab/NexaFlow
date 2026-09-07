# NexaFlow production roadmap

This roadmap is derived from the repository as of 2026-08-27. It treats the
current product as a credible MVP and sequences enterprise work by dependency
and operational risk, not by UI visibility.

## Current implementation baseline

| Capability | Current evidence | Production gap |
| --- | --- | --- |
| Multi-tenancy | Tenant memberships, tenant roles, token switching, tenant-scoped aggregates, request tenant context, PostgreSQL RLS | Cross-tenant automated integration suite, tenant export/purge workflow, custom-domain routing |
| Authentication | JWT access tokens, rotating hashed refresh sessions, device metadata, revocation | OAuth/OIDC, SAML, MFA/WebAuthn, recovery, lockout, configurable password/session policies enforced at login |
| Authorization | Tenant RBAC, route middleware, dynamic navigation | ABAC policy evaluation, field masking, row predicates, decision audit |
| Dynamic data | Entity schemas, JSONB records, forms, optimistic concurrency | Views, formulas, bounded query language, import/export, indexing advisor |
| Workflow | Node graphs, approval actions, instances, history, notification outbox | Immutable published versions, timers, parallel branches, retries/DLQ per action, AI/action nodes |
| AI and knowledge | Conversations, permission-aware tools, pgvector search, durable ingestion jobs, citations at chunk level | Agent registry, tool registry, OCR/virus scan, hybrid retrieval, reranking, page/paragraph provenance, evaluation |
| Files | S3-compatible object store, metadata, access checks | Malware scanning, quarantine, reconciliation, retention, versioning controls |
| Billing | Plans, Stripe checkout/webhook, tenant quotas and usage | Entitlement ledger, metering reconciliation, invoice/tax operations, webhook replay console |
| Analytics | Dashboard widgets over bounded aggregates | Report model, safe SQL compiler, chart library, scheduled delivery |
| Operations | OpenTelemetry, SLOs, Redis-backed distributed jobs/rate limits, hardened Compose/Kubernetes, signed encrypted backup/restore drills, security scans | Multi-region failover and continuous recovery-objective evidence |
| Extensibility | Documented extension seams | Signed manifests, permission sandbox, route/migration registry, lifecycle manager, CLI generators |

The backend currently has 17 service interfaces, 70+ HTTP routes and 24 Go
test files. Frontend lint, type checking and production build pass. Docker
Compose cannot be validated in the present workstation because Docker is not
installed.

## Delivery rules

Every module must preserve `HTTP -> handler -> service -> repository ->
database`. Its interface is the test surface; implementation details remain
behind that seam. A module is complete only when it includes:

- an immutable forward migration and rollback/risk note;
- versioned REST contracts and permission decisions;
- unit tests plus tenant-isolation and failure-path integration tests;
- operational metrics, structured audit events and runbook changes;
- updated API, database, security and deployment documentation.

No enterprise customer pilot begins until P0 exits are met.

## P0 — isolation and release safety

### M1. Enterprise tenant foundation — implemented

- Verified `TenantContext` carries tenant, user, primary role, roles and
  permissions in `context.Context`.
- Repository calls fail closed without tenant or explicit system context.
- Each repository call runs in a transaction that sets tenant/user identity
  with transaction-local PostgreSQL settings.
- RLS is enabled and forced on every tenant-owned table; policies cover reads,
  writes and raw SQL.
- Tenant profile/settings, optimistic versions, suspend/archive/delete
  lifecycle and session revocation are exposed through permission-protected
  endpoints.

Exit work: add a real PostgreSQL integration matrix proving tenant A cannot
select/update/delete/insert tenant B data through GORM and raw SQL; add an
operator-owned delayed purge/export workflow before calling deletion GDPR-ready.

### M2. Migration and CI hardening — implemented

- Extract migrations from the installer into reviewed immutable migration
  files with checksums, advisory locking and expand/migrate/contract guidance.
- Add PostgreSQL/Redis/S3 integration jobs, browser smoke tests, race detector,
  dependency/license scan, secret scan, SAST and container scan.
- Produce SBOMs and signed multi-architecture images; define rollback and
  compatibility policy.

### M3. Observability and distributed jobs — implemented

- Add OpenTelemetry traces and metrics for HTTP, GORM, Redis, object storage,
  AI providers and workflow execution.
- Publish API/database latency, error rate, pool saturation, queue depth,
  retries, DLQ size, AI token/cost and quota-denial metrics.
- Replace process-local rate limiting and polling workers with Redis-backed
  durable queues supporting priority, idempotency, retry and DLQ replay.
- Define SLOs, alerts, dashboards and incident runbooks.

### M4. Production deployment and recovery — implemented

- Add hardened production Compose and Kubernetes reference deployment with
  Nginx/ingress TLS, non-root containers, resource limits and graceful drains.
- Automate daily encrypted PostgreSQL backups and S3/R2 versioning; run and
  record scheduled restore tests with RPO/RTO targets.
- Add zero-downtime deployment checks, readiness dependencies and disaster
  recovery exercises.

## P1 — enterprise identity and authorization

### M5. Enterprise authentication

- Add provider-neutral OAuth2/OIDC adapters for Google, Microsoft and GitHub.
- Add tenant-configurable SAML 2.0 SSO, signed metadata, certificate rotation,
  just-in-time provisioning and domain discovery.
- Enforce tenant password policy, attempt throttling, account lockout,
  MFA/WebAuthn, recovery codes, session limits and security-event audit.
- Add invitations, email verification, password reset and SCIM provisioning.

### M6. RBAC plus ABAC

- Introduce a policy-decision module over subject, tenant, resource, action and
  environment attributes; keep RBAC as the first coarse filter.
- Add row predicates and field-level read/write masks with deny-by-default
  composition, policy simulation and decision traces.
- Normalize permissions to resource actions such as `customer.read`,
  `order.approve` and `document.export`, with compatibility aliases during
  migration.

### M7. Security and privacy hardening

- Add request schemas, CSRF protection for cookie flows, distributed rate
  limits, file size/type policy, malware scan and quarantine.
- Extend audit events to before/after snapshots with redaction, retention,
  export and immutable archive options.
- Add tenant data export, legal hold, retention and delayed cryptographic purge;
  perform threat modeling and an external penetration test before GA.

## P2 — workflow and AI-native platform

### M8. Enterprise workflow engine

- Separate editable drafts from immutable published workflow versions; pin each
  instance to its starting version.
- Add condition, approval, AI decision, action, notification and end nodes with
  typed inputs/outputs and per-node permission checks.
- Add timers, escalation, delegation, parallel joins, idempotent action keys,
  retries, DLQ, compensation hooks and complete duration/error history.

### M9. Agent and tool platform

- Add tenant-scoped Agent definitions for prompts, tools, permissions,
  knowledge sources, model policy, budgets and versions.
- Add a typed Tool Registry (`name`, schema, description, permission,
  idempotency, handler) whose executor reauthorizes every call.
- Persist tool request/result/error/cost audit and support approval gates for
  consequential actions. Start with Customer, HR, Sales and Finance templates.

### M10. Advanced RAG pipeline

- Implement upload -> quarantine -> virus scan -> extract/OCR -> normalize ->
  chunk -> embed -> index as durable, observable jobs.
- Support PDF, DOCX, XLSX and images through replaceable extractors.
- Combine PostgreSQL full-text and vector search, add reranking, deduplication,
  document versions and re-indexing.
- Require validated citations with document, page, paragraph and immutable
  source version; add retrieval and prompt-injection evaluation suites.

## P3 — enterprise data and analytics

### M11. Metadata-driven data engine

- Add versioned metadata for entity fields, relations, validation, indexes and
  saved table/Kanban/calendar views.
- Implement a sandboxed formula language with dependency graphs, deterministic
  recalculation and type checking.
- Add bounded filtering/sorting, bulk actions, CSV import/export and online
  JSONB index recommendations.

### M12. Reporting and analytics

- Add report metadata and a parameterized SQL compiler that emits only
  allowlisted, tenant-scoped query plans.
- Support line, bar, pie and funnel widgets, drill-down and cached aggregates.
- Add scheduled reports through the job system with timezone-aware delivery,
  permission snapshots and auditable exports.

## P4 — ecosystem and developer experience

### M13. Plugin architecture

- Define signed `manifest.json`, compatibility range, permissions, routes,
  migrations, settings and lifecycle hooks.
- Add install/enable/disable/upgrade/rollback management, route namespaces and
  database ownership rules. Plugins may use documented seams only and cannot
  bypass authorization or tenant context.
- Validate with first-party CRM, ERP and HR reference plugins.

### M14. Developer CLI and SDK

- Ship `nexaflow create-app`, `generate-entity` and `migrate` with deterministic
  templates for backend slice, frontend page, permissions, migrations and tests.
- Generate OpenAPI, typed TypeScript/Go clients and plugin SDK conformance tests.

### M15. Enterprise GA

- Run scale, chaos, failover, tenant-isolation and upgrade tests against stated
  capacity targets.
- Complete accessibility, localization, support, privacy, compliance evidence,
  upgrade policy and customer-facing operational documentation.
- Pilot with design partners behind explicit feature flags, then graduate only
  after SLO and recovery objectives are demonstrated.

## Recommended next module

Proceed with M5: enterprise authentication. The platform now has release,
observability, durable execution and recovery foundations needed to operate
identity migrations and security events safely.
