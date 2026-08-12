# REST API

Every versioned endpoint uses the response envelope:

```json
{ "code": 0, "message": "success", "data": {} }
```

## Installation endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/install/status` | Return version and lock state. |
| `GET` | `/api/v1/install/environment` | Run live environment and dependency checks. |
| `GET` | `/api/v1/install/readiness` | Return infrastructure checks and secret-safe optional capability status. |
| `POST` | `/api/v1/install/complete` | Initialize the first tenant and administrator using server-side infrastructure configuration. |

### Complete request

```json
{
  "admin": {
    "username": "admin",
    "email": "admin@example.com",
    "password": "at-least-12-characters"
  },
  "company": {
    "name": "Example Company",
    "industry": "manufacturing",
    "default_language": "zh-CN",
    "timezone": "Asia/Shanghai"
  }
}
```

Installation errors use codes `2002`-`2005`. Infrastructure details are
returned only as health/remediation status; credentials never cross the
browser boundary.

## Authentication and RBAC endpoints

| Method | Path | Authorization |
| --- | --- | --- |
| `POST` | `/api/v1/auth/register` | Public; assigns `employee`. |
| `POST` | `/api/v1/auth/login` | Public. |
| `POST` | `/api/v1/auth/refresh` | Opaque refresh token. |
| `POST` | `/api/v1/auth/switch-tenant` | Membership-checked refresh rotation. |
| `POST` | `/api/v1/auth/logout` | Opaque refresh token. |
| `GET` | `/api/v1/auth/me` | Bearer access token. |
| `GET` | `/api/v1/auth/menu` | Bearer access token. |
| `GET` | `/api/v1/auth/tenants` | Active memberships. |
| `POST` | `/api/v1/auth/tenants` | Create owned tenant. |
| `GET` | `/api/v1/auth/sessions` | Bearer access token. |
| `DELETE` | `/api/v1/auth/sessions/:sessionID` | Session owner. |
| `GET` | `/api/v1/auth/users` | `user.view`. |
| `GET` | `/api/v1/auth/roles` | `role.manage`. |
| `GET` | `/api/v1/auth/permissions` | `role.manage`. |
| `PUT` | `/api/v1/auth/roles/:roleID/permissions` | `role.manage`. |
| `PUT` | `/api/v1/auth/users/:userID/roles` | `role.manage`. |

Send access tokens as `Authorization: Bearer <token>`. Authentication and
authorization failures use error codes `3001`-`3113` without exposing password,
token, or database details.

Protected requests may include `X-Tenant-ID`, but it must match the JWT tenant.
Switching tenants requires both `tenant_id` and the current `refresh_token`; a
successful switch returns a completely rotated token pair.

## Dynamic entity endpoints

All dynamic-model endpoints require an access token with an active tenant claim.
The server derives tenant scope from that claim; clients cannot select another
tenant by putting a tenant identifier in the request body.

| Method | Path | Authorization | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/entities` | `entity.view` | List active entity schemas. |
| `GET` | `/api/v1/entities/:entityID` | `entity.view` | Read one schema and its ordered fields. |
| `POST` | `/api/v1/entities` | `entity.manage` | Define an entity schema. |
| `PUT` | `/api/v1/entities/:entityID` | `entity.manage` | Replace a complete schema using optimistic concurrency. |
| `DELETE` | `/api/v1/entities/:entityID?expected_version=N` | `entity.manage` | Archive a schema. |

Create and update accept a complete schema definition:

```json
{
  "name": "客户",
  "slug": "customers",
  "description": "销售与服务共享的客户主数据",
  "expected_version": 2,
  "fields": [
    { "name": "display_name", "label": "客户名称", "type": "string", "required": true, "position": 0 },
    { "name": "tier", "label": "客户等级", "type": "select", "required": false, "options": ["A", "B", "C"], "default": "B", "position": 1 }
  ]
}
```

`expected_version` is omitted on create and required on update/archive. A stale
version returns HTTP `409` with code `4105`; a tenant-local duplicate slug uses
`4106`. Validation, missing-schema, and internal errors use codes `4101`-`4199`.
Supported field types are `string`, `text`, `number`, `boolean`, `date`,
`datetime`, `select`, `multiselect`, `money`, `email`, `url`, `user`, `image`,
and `attachment`.

## Generated record CRUD

Record endpoints are generated from an entity definition rather than from
runtime Go code or PostgreSQL DDL. Every request is tenant-scoped and values are
validated against the active schema before reaching persistence.

| Method | Path | Authorization |
| --- | --- | --- |
| `GET` | `/api/v1/entities/:entityID/records?page=1&page_size=25` | `record.view` |
| `POST` | `/api/v1/entities/:entityID/records` | `record.manage` |
| `GET` | `/api/v1/entities/:entityID/records/:recordID` | `record.view` |
| `PUT` | `/api/v1/entities/:entityID/records/:recordID` | `record.manage` |
| `DELETE` | `/api/v1/entities/:entityID/records/:recordID?expected_version=N` | `record.manage` |

Write requests use `{ "values": { ... }, "expected_version": 2 }`.
`expected_version` is required for update/delete. Unknown fields, missing
required values, wrong JSON types, invalid dates/URLs/emails, and select values
outside the entity options are rejected. Create applies schema defaults. List
returns `{ items, total, page, page_size }`; page size is capped at 100.
Record errors use codes `4201`-`4299`, including `4206` for a stale version.

## Low-code forms

| Method | Path | Authorization |
| --- | --- | --- |
| `GET` | `/api/v1/forms?entity_id=:entityID` | `form.view` |
| `POST` | `/api/v1/forms` | `form.manage` |
| `GET` | `/api/v1/forms/:formID` | `form.view` |
| `PUT` | `/api/v1/forms/:formID` | `form.manage` |
| `DELETE` | `/api/v1/forms/:formID?expected_version=N` | `form.manage` |

A form contains ordered components with `field_name`, `widget`, `label`,
`required`, and optional `props`. The service verifies every component against
the referenced entity, normalizes positions, and generates JSON Schema draft
2020-12 with `additionalProperties: false`. Saved forms cannot be rebound to a
different entity. Form errors use codes `4301`-`4399`.

## Workflows, approvals, and notifications

| Method | Path | Authorization |
| --- | --- | --- |
| `GET` / `POST` | `/api/v1/workflows` | `workflow.view` / `workflow.manage` |
| `GET` / `PUT` / `DELETE` | `/api/v1/workflows/:workflowID` | View/manage; delete uses `expected_version`. |
| `POST` | `/api/v1/workflows/:workflowID/start` | `workflow.submit` |
| `GET` | `/api/v1/workflow-instances?workflow_id=:id` | `workflow.view` |
| `POST` | `/api/v1/workflow-instances/:instanceID/actions` | `workflow.approve` plus node role |
| `GET` | `/api/v1/notifications` | Authenticated user in current tenant |
| `PUT` | `/api/v1/notifications/:notificationID/read` | Notification owner |

Definitions contain `start`, `approval`, `condition`, `notification`, and
`end` nodes plus directed edges. The server rejects cycles, unreachable nodes,
missing approval roles, and conditions without exactly one `true` and one
`false` branch. Start accepts `{ "record_id": "..." }` and verifies record,
entity, and tenant ownership.

Actions accept `approve` or `reject`, a comment, and `expected_version`.
Authorization combines live `workflow.approve` permission with the current
node's `assignee_role`; super administrators may act on any approval. Workflow
and instance conflicts use codes `4408` and `4411`.

In-app notification nodes target the workflow submitter. Email and webhook
nodes require an explicit recipient and create durable `pending` outbox rows;
they are not reported as delivered before a dispatcher succeeds.

## Knowledge and AI Agent

| Method | Path | Authorization |
| --- | --- | --- |
| `GET` / `POST` | `/api/v1/knowledge/documents` | `knowledge.view` / `knowledge.manage` |
| `DELETE` | `/api/v1/knowledge/documents/:documentID` | `knowledge.manage` |
| `POST` | `/api/v1/knowledge/search` | `knowledge.search` |
| `POST` | `/api/v1/ai/ask` | `ai.chat` and live tool permissions |
| `GET` | `/api/v1/ai/conversations` | Conversation owner |
| `GET` | `/api/v1/ai/conversations/:conversationID/messages` | Conversation owner |

Knowledge upload is multipart field `file`, limited to 20 MiB. Supported
formats are PDF, DOCX, XLSX, and UTF-8 TXT/MD/CSV/JSON. Text is parsed,
normalized, chunked with overlap, embedded, and committed atomically. Scanned
PDFs require OCR before upload; empty extraction is rejected.

`/ai/ask` accepts `{ "conversation_id": "optional", "message": "..." }`.
The Agent always performs tenant-scoped knowledge search. With live
`record.view`, an entity name/slug in the question may trigger a bounded record
read. Tool metadata, sources, and token use are persisted. Without `AI_API_KEY`,
the endpoint returns `503` / code `4505`; no mock answer is generated.

## Files and dashboards

| Method | Path | Authorization |
| --- | --- | --- |
| `GET` / `POST` | `/api/v1/files` | `file.view` / `file.manage` |
| `GET` | `/api/v1/files/:fileID/download` | `file.view` |
| `DELETE` | `/api/v1/files/:fileID` | `file.manage` |
| `GET` / `PUT` | `/api/v1/dashboard` | `dashboard.view` / `dashboard.manage` |

File upload uses multipart field `file`, accepts images, PDF, XLS/XLSX, and is
limited to 50 MiB. Metadata and object keys are tenant scoped. Dashboard widgets
support active-user count, file count, entity record count, and numeric/money
field sum. The server validates entity and field IDs and never accepts SQL.

## Plans and billing

| Method | Path | Authorization |
| --- | --- | --- |
| `GET` | `/api/v1/billing/plans` | Authenticated tenant member |
| `GET` | `/api/v1/billing/overview` | `billing.manage` |
| `POST` | `/api/v1/billing/checkout` | `billing.manage` |
| `POST` | `/api/v1/billing/webhook` | Stripe signature verification |

Record count, knowledge bytes, and AI tokens are enforced atomically against
the active tenant plan. Stripe subscription events require a valid HMAC
signature within five minutes, an allowlisted event type, tenant metadata, and
a configured Price ID. Event IDs are persisted for idempotency.
