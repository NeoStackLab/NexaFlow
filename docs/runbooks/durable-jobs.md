# Durable jobs incident runbook

## Queue depth or age is rising

1. Check Redis readiness, latency, memory pressure and rejected connections.
2. Split dashboard panels by job kind. Confirm workers are exporting completed
   or retry counters and that deployments have matching handler registrations.
3. Check PostgreSQL pool saturation because both built-in handlers claim their
   durable domain records from PostgreSQL.
4. Scale worker replicas or `JOBS_CONCURRENCY` only after Redis and PostgreSQL
   have headroom. Priority 9 is served before priority 0.
5. If leases repeatedly expire, compare handler duration with
   `JOBS_LEASE_TIMEOUT`; fix or split the handler before increasing the lease.

## DLQ is non-empty

1. Identify the job kind and correlate its job ID with the consumer trace.
2. Correct credentials, downstream availability, payload handling or the
   incompatible deployment. Do not replay while the same error is active.
3. Replay at most 100 jobs using the bounded `ReplayDLQ` operation.
4. Verify completion, retry and 5xx signals for 15 minutes; continue in bounded
   batches. Retain failed envelopes until the incident record is complete.

## Redis is unavailable

- Authentication endpoints fail closed with HTTP 503 instead of bypassing the
  distributed rate limit.
- Durable jobs remain in Redis AOF storage; API instances stop claiming work.
- Restore Redis connectivity, confirm `PING`, then watch lease reclaim and queue
  depth. Do not flush or rename the configured job namespace.

## Rollback

Application rollback is safe while old and new releases recognize the same job
kinds and envelope fields. Drain workers before a release that removes a
handler. Redis keys are data, not a cache: never use `FLUSHDB` during rollback.
