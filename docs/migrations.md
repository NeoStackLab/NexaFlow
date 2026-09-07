# Database migration operations

NexaFlow uses the deep `internal/migration` module as the only schema upgrade
entry point. Install and normal startup both call `DefaultRunner().Run`; HTTP
handlers and repositories do not know migration ordering or ledger details.

## Safety contract

- Migrations are append-only, contiguous, and ordered from version 1. Never
  edit, reorder, rename, or delete a migration that may have reached a shared
  environment.
- Each migration has an embedded immutable specification under
  `backend/internal/migration/specs`. Its SHA-256 digest and name are recorded
  in `schema_migrations`; startup fails closed if either differs.
- A PostgreSQL session advisory lock serializes all migration runners. Every
  pending migration and its ledger insert use one transaction, so a failure
  rolls back both schema/data changes and version advancement.
- The application connection must be `NOSUPERUSER NOBYPASSRLS`. Migrations use
  a transaction-local system context for intentional tenant-wide backfills;
  that context cannot leak through the pool after commit.
- A database containing unknown, non-contiguous, or checksum-mismatched history
  is not repaired automatically. Stop deployment, preserve the database, and
  investigate the binary/schema pairing.

The ledger extends older installations in place with `checksum` and
`duration_ms`; the first run of the new runner backfills trusted checksums for
legacy versions 1-6 after verifying their recorded names.

## Adding a migration

1. Add `specs/NNNNNN_descriptive_name.txt` with forward behavior, compatibility
   window, operational risk, verification, and rollback/restore guidance.
2. Add one apply function and append exactly one `migrationFromSpec` entry to
   `defaultMigrations`. Keep the apply function deterministic and idempotent
   enough for operator diagnosis, but rely on the transaction and ledger for
   exactly-once version advancement.
3. Add unit tests for validation and failure behavior. If the migration touches
   PostgreSQL semantics such as RLS, indexes, locks, extensions, or JSON/vector
   operators, add a real PostgreSQL integration test—not only SQL mocks.
4. Run `go test ./...`, `go test -tags=integration ./internal/migration
   ./internal/integration`, `go test -race ./...`, and `go vet ./...`.
5. Deploy the migration-capable release before code that requires the final
   schema contract, and observe duration, errors, locks, and database capacity.

## Expand, migrate, contract

Zero-downtime changes span releases:

1. **Expand:** add nullable columns/tables/indexes without removing old reads or
   writes. Prefer PostgreSQL online/concurrent techniques for measured large
   tables; document any statement that cannot run in the runner transaction.
2. **Migrate:** dual-write where necessary, backfill in bounded restartable
   batches, verify counts/checksums, then switch reads behind a feature flag.
3. **Contract:** only after every supported application version no longer uses
   the old shape, make constraints strict and remove obsolete schema.

Do not combine an irreversible destructive contract with the first release
that stops using the old field. Roll application code forward when possible.
For a failed transactional migration, fix the migration in a new unreleased
build only if it has never been applied anywhere; once applied, append a new
corrective migration. For production data loss or a committed destructive
change, restore into a new database from the tested backup/PITR point and
follow the incident runbook rather than deleting ledger rows.
