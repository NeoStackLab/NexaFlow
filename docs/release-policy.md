# Release, compatibility, and rollback policy

## CI release gates

Pull requests must pass Go unit/integration/race/vet checks, frontend
lint/type/build checks, a full Compose browser smoke test, dependency and
license review, secret/misconfiguration scanning, CodeQL SAST, and built-image
vulnerability scanning. PostgreSQL 18 + pgvector, Redis 8, and an S3-compatible
LocalStack endpoint exercise the actual infrastructure adapters.

Version tags build backend and frontend images for `linux/amd64` and
`linux/arm64`, publish them to GHCR, attach BuildKit provenance/SBOM
attestations, upload SPDX JSON SBOM artifacts, and keylessly sign the immutable
manifest digest with Cosign/GitHub OIDC. Production promotion must select the
digest, not a mutable tag, and verify signature and provenance before rollout.

## Compatibility window

- A release supports the schema produced by its own migrations and the
  immediately previous application release during rolling deployment.
- Database changes follow expand/migrate/contract. A contract migration cannot
  ship until the oldest supported application release has been drained.
- Public API removals and incompatible response changes require a new API
  version or a documented deprecation window. Additive fields are preferred.
- Workers and HTTP instances must use the same major/minor release during a
  rollout. Queue payloads need explicit versions before mixed-version workers
  are supported.
- PostgreSQL 18, Redis 8, and the documented pgvector version are the current
  infrastructure contract. Changing a major version requires an upgrade matrix
  and restore rehearsal.

## Rollback decision

1. Stop promotion when readiness, migration, smoke, or SLO gates fail.
2. If only application code changed and the schema remains backward compatible,
   redeploy the previous signed image digest.
3. If an expand migration committed, keep the additive schema and roll the
   application back; do not delete the migration ledger row.
4. If a migrate step partially completed outside a transaction, stop writers,
   resume or reverse using its documented idempotent procedure, and verify data
   before reopening traffic.
5. If a contract migration or data corruption prevents application rollback,
   restore PostgreSQL to a new instance from the last verified backup/PITR
   point, validate tenant isolation and counts, then switch traffic deliberately.

Rollback authority, incident communication, RPO/RTO, and restore-test evidence
remain operator-owned. Image rollback is not a substitute for database backup.
