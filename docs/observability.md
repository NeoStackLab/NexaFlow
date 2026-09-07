# Observability and reliability

NexaFlow emits vendor-neutral traces and metrics through OTLP/HTTP. Set
`OBSERVABILITY_ENABLED=true` and point `OBSERVABILITY_ENDPOINT` at an
OpenTelemetry Collector. The endpoint is a host and port (for example
`otel-collector:4318`); TLS is controlled by `OBSERVABILITY_INSECURE`.

## Signal coverage

| Surface | Traces | Metrics |
| --- | --- | --- |
| HTTP API | server spans with route and status | request duration and 5xx count |
| PostgreSQL/GORM | query spans | query duration plus open, in-use and wait-count pool gauges |
| Redis | command spans | command duration and connection-pool instruments |
| Object storage | put/open/delete spans | operation duration by provider and operation |
| AI provider | embedding/completion spans | provider duration, input/output tokens and estimated USD cost |
| Workflow | start/advance spans | trace status and execution attributes |
| Durable jobs | producer/consumer spans with propagated context | depth, duration, retries, completions and DLQ depth |
| Tenant quotas | request trace context | denial count by quota metric |

AI cost is an estimate. Configure `AI_INPUT_COST_PER_MILLION_TOKENS` and
`AI_OUTPUT_COST_PER_MILLION_TOKENS` for the selected model; zero keeps cost
reporting enabled without inventing a price.

## Service-level objectives

The initial production objectives use a rolling 30-day window:

- API availability: 99.9% of non-health requests are not 5xx.
- API latency: 95% below 500 ms and 99% below 2 seconds.
- Durable-job latency: 99% of ready priority 5-9 work begins within 60 seconds.
- Durable-job success: 99.5% completes without reaching the DLQ.
- Telemetry delivery: collector export failures stay below 0.1% over 15 minutes.

Page on a 14.4x burn rate over one hour or a 6x burn rate over six hours. Ticket
slower burns, pool saturation above 80% for 15 minutes, any sustained DLQ growth,
or a ready queue older than the job-latency objective.

## Collector assets

`docker/observability/otel-collector.yaml` receives OTLP and exposes Prometheus
metrics on port 8889. `prometheus.yaml` loads `alerts.yaml`, while
`nexaflow-dashboard.json` is an importable Grafana dashboard. Pin container
images in the deployment repository and run these assets behind the production
monitoring network rather than exposing collector ports publicly.

## Distributed job behavior

Redis stores job envelopes, ten priority queues, delayed and processing sorted
sets, idempotency keys, and the DLQ. Claims and state transitions are Lua-atomic.
Workers reclaim expired leases after a crash, retry with bounded exponential
backoff, and preserve trace context across enqueue and execution. Periodic
knowledge-ingestion and notification dispatch are scheduled through the same
runtime, so adding API replicas does not create process-local polling loops.

DLQ replay is intentionally an operator action through `Queue.ReplayDLQ`; fix
the underlying fault first, replay a bounded batch, and watch error rate and
queue age before continuing.
