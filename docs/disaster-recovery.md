# Disaster recovery

## Objectives and protection boundary

The initial production targets are:

| Objective | Target | Evidence |
| --- | --- | --- |
| Database RPO | 24 hours or less | Daily successful encrypted backup |
| Database RTO | 4 hours or less | Weekly isolated restore duration and quarterly exercise |
| Restore-test freshness | 8 days or less | Signed restore evidence used by the release gate |
| Backup retention | 35 days | Object-store retention/versioning configuration |

These are operating targets, not guarantees. Customers needing a smaller RPO
must use managed PostgreSQL point-in-time recovery in addition to the portable
daily archive. Database backups do not include user objects: enable
versioning/retention for the application object bucket and replicate it when
the business impact requires region-loss recovery. Redis is rebuildable cache
state, but queued jobs may be lost unless its managed persistence and recovery
are configured.

## Backup format and trust boundary

`docker/backup/backup.sh create` produces a PostgreSQL custom archive, validates
its table of contents, encrypts it to an age recipient, hashes the ciphertext,
and publishes a minisign-signed JSON manifest. The archive and signature are
uploaded before the manifest, so the manifest is the publication marker.
Timestamped object keys are never reused.

Keep the age identity offline from the normal application runtime. The backup
writer needs only the public age recipient and the manifest signing private
key. Restore operators need the age identity and signing public key. Rotate
keys by retaining old identities/public keys until every archive encrypted or
signed by them has expired.

For AWS S3, run:

```sh
BACKUP_BUCKET=nexaflow-backups sh scripts/configure-object-retention.sh s3
```

This enables native bucket versioning; configure lifecycle retention and, when
required, Object Lock separately. Cloudflare R2 does not use this script for
versioning. Instead, immutable timestamped keys are protected with an R2 bucket
lock rule:

```sh
BACKUP_BUCKET=nexaflow-backups BACKUP_PREFIX=database/ \
  BACKUP_RETENTION_DAYS=35 sh scripts/configure-object-retention.sh r2
```

## Restore drill

The weekly timer/CronJob runs `restore-latest-drill`. It verifies the signed
manifest and ciphertext hash, decrypts the archive, creates a new scratch
database, restores with `--exit-on-error`, validates migration and tenant
tables, records signed JSON evidence, and drops the scratch database. It never
overwrites the live database.

Alert on failed backup or drill Jobs, stale evidence, missing objects and a
duration trend approaching the four-hour RTO. A passed CI drill proves the
tooling; only a scheduled drill against production-shaped backups proves the
operational target.

## Recovery procedure

1. Declare the incident, freeze deployments and preserve logs/traces.
2. Identify the incident timestamp and newest valid manifest before data
   corruption. Verify the signature and retention status.
3. Provision an isolated PostgreSQL 18 target with pgvector and sufficient
   capacity. Never restore over the affected database.
4. Run the recovery image with `restore <manifest-uri>
   nexaflow_recovery_<incident>` and set
   `RESTORE_CONFIRM=restore:nexaflow_recovery_<incident>`.
5. Validate migration ledger, tenant counts, critical records, object links and
   application smoke tests. Record actual RPO and RTO.
6. Put the application in maintenance mode, repoint it to the recovered
   database, start one backend replica, then progressively restore traffic.
7. Reconcile object storage and durable jobs, revoke exposed credentials, and
   retain the old database read-only for investigation.

Perform a quarterly region-loss tabletop and hands-on exercise covering
database loss, object deletion, credential compromise and bad deployment.
Record owners, timestamps, decisions, measured RPO/RTO and corrective actions.
