package observability

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

type Runtime struct {
	traces  *sdktrace.TracerProvider
	metrics *sdkmetric.MeterProvider
	Metrics *Metrics
}

type Metrics struct {
	httpDuration metric.Float64Histogram
	httpErrors   metric.Int64Counter
	jobEnqueued  metric.Int64Counter
	jobCompleted metric.Int64Counter
	jobRetried   metric.Int64Counter
	jobDead      metric.Int64Counter
	jobDuration  metric.Float64Histogram
	jobReady     metric.Int64Gauge
	jobDLQ       metric.Int64Gauge
}

func Start(ctx context.Context, cfg config.ObservabilityConfig, serviceName, serviceVersion, environment string) (*Runtime, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", serviceVersion),
		attribute.String("deployment.environment.name", environment),
	))
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}
	var traceOptions []sdktrace.TracerProviderOption
	traceOptions = append(traceOptions, sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))))
	var metricOptions []sdkmetric.Option
	metricOptions = append(metricOptions, sdkmetric.WithResource(res))
	if cfg.Enabled {
		traceExporterOptions := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
		metricExporterOptions := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			traceExporterOptions = append(traceExporterOptions, otlptracehttp.WithInsecure())
			metricExporterOptions = append(metricExporterOptions, otlpmetrichttp.WithInsecure())
		}
		traceExporter, exporterErr := otlptracehttp.New(ctx, traceExporterOptions...)
		if exporterErr != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", exporterErr)
		}
		metricExporter, exporterErr := otlpmetrichttp.New(ctx, metricExporterOptions...)
		if exporterErr != nil {
			return nil, fmt.Errorf("create OTLP metric exporter: %w", exporterErr)
		}
		traceOptions = append(traceOptions, sdktrace.WithBatcher(traceExporter))
		metricOptions = append(metricOptions, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(cfg.MetricInterval))))
	}
	traces := sdktrace.NewTracerProvider(traceOptions...)
	meters := sdkmetric.NewMeterProvider(metricOptions...)
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(meters)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	metrics, err := newMetrics(meters.Meter("nexaflow/runtime"))
	if err != nil {
		_ = traces.Shutdown(ctx)
		_ = meters.Shutdown(ctx)
		return nil, err
	}
	return &Runtime{traces: traces, metrics: meters, Metrics: metrics}, nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	return errors.Join(r.traces.Shutdown(ctx), r.metrics.Shutdown(ctx))
}

func newMetrics(meter metric.Meter) (*Metrics, error) {
	metrics := &Metrics{}
	var err error
	if metrics.httpDuration, err = meter.Float64Histogram("nexaflow.api.request.duration", metric.WithUnit("s")); err != nil {
		return nil, err
	}
	if metrics.httpErrors, err = meter.Int64Counter("nexaflow.api.request.errors"); err != nil {
		return nil, err
	}
	if metrics.jobEnqueued, err = meter.Int64Counter("nexaflow.jobs.enqueued"); err != nil {
		return nil, err
	}
	if metrics.jobCompleted, err = meter.Int64Counter("nexaflow.jobs.completed"); err != nil {
		return nil, err
	}
	if metrics.jobRetried, err = meter.Int64Counter("nexaflow.jobs.retries"); err != nil {
		return nil, err
	}
	if metrics.jobDead, err = meter.Int64Counter("nexaflow.jobs.dead_lettered"); err != nil {
		return nil, err
	}
	if metrics.jobDuration, err = meter.Float64Histogram("nexaflow.jobs.duration", metric.WithUnit("s")); err != nil {
		return nil, err
	}
	if metrics.jobReady, err = meter.Int64Gauge("nexaflow.jobs.depth"); err != nil {
		return nil, err
	}
	if metrics.jobDLQ, err = meter.Int64Gauge("nexaflow.jobs.dlq.depth"); err != nil {
		return nil, err
	}
	return metrics, nil
}

func (m *Metrics) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := otel.Tracer("nexaflow/http").Start(ctx, c.Request.Method+" "+c.FullPath(), trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attribute.String("http.request.method", c.Request.Method), attribute.String("url.path", c.Request.URL.Path)))
		c.Request = c.Request.WithContext(ctx)
		defer span.End()
		c.Next()
		status := c.Writer.Status()
		attrs := metric.WithAttributes(attribute.String("http.request.method", c.Request.Method), attribute.String("http.route", c.FullPath()), attribute.Int("http.response.status_code", status))
		m.httpDuration.Record(ctx, time.Since(started).Seconds(), attrs)
		if status >= http.StatusInternalServerError {
			m.httpErrors.Add(ctx, 1, attrs)
			span.SetStatus(codes.Error, strconv.Itoa(status))
		}
		span.SetAttributes(attribute.Int("http.response.status_code", status))
	}
}

func (m *Metrics) Enqueued(ctx context.Context, kind string) {
	m.jobEnqueued.Add(ctx, 1, metric.WithAttributes(attribute.String("job.kind", kind)))
}
func (m *Metrics) Completed(ctx context.Context, kind string, duration time.Duration) {
	attrs := metric.WithAttributes(attribute.String("job.kind", kind))
	m.jobCompleted.Add(ctx, 1, attrs)
	m.jobDuration.Record(ctx, duration.Seconds(), attrs)
}
func (m *Metrics) Retried(ctx context.Context, kind string) {
	m.jobRetried.Add(ctx, 1, metric.WithAttributes(attribute.String("job.kind", kind)))
}
func (m *Metrics) DeadLettered(ctx context.Context, kind string) {
	m.jobDead.Add(ctx, 1, metric.WithAttributes(attribute.String("job.kind", kind)))
}
func (m *Metrics) Depth(ctx context.Context, ready, dead int64) {
	m.jobReady.Record(ctx, ready)
	m.jobDLQ.Record(ctx, dead)
}
func InstrumentRedis(client redis.UniversalClient) error {
	return errors.Join(redisotel.InstrumentTracing(client), redisotel.InstrumentMetrics(client))
}

func InstrumentDatabase(db *gorm.DB, pool *sql.DB) error {
	if err := db.Use(tracing.NewPlugin()); err != nil {
		return fmt.Errorf("instrument GORM: %w", err)
	}
	meter := otel.Meter("nexaflow/database")
	duration, err := meter.Float64Histogram("nexaflow.database.operation.duration", metric.WithUnit("s"))
	if err != nil {
		return err
	}
	operationErrors, err := meter.Int64Counter("nexaflow.database.operation.errors")
	if err != nil {
		return err
	}
	if err := db.Use(&databaseMetricsPlugin{duration: duration, errors: operationErrors}); err != nil {
		return fmt.Errorf("instrument database metrics: %w", err)
	}
	open, err := meter.Int64ObservableGauge("nexaflow.database.connections.open")
	if err != nil {
		return err
	}
	inUse, err := meter.Int64ObservableGauge("nexaflow.database.connections.in_use")
	if err != nil {
		return err
	}
	waiting, err := meter.Int64ObservableCounter("nexaflow.database.connections.wait_count")
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		stats := pool.Stats()
		observer.ObserveInt64(open, int64(stats.OpenConnections))
		observer.ObserveInt64(inUse, int64(stats.InUse))
		observer.ObserveInt64(waiting, stats.WaitCount)
		return nil
	}, open, inUse, waiting)
	return err
}

type databaseMetricsPlugin struct {
	duration metric.Float64Histogram
	errors   metric.Int64Counter
}

func (*databaseMetricsPlugin) Name() string { return "nexaflow-database-metrics" }

func (p *databaseMetricsPlugin) Initialize(db *gorm.DB) error {
	callbacks := db.Callback()
	registrations := []struct {
		operation string
		before    interface {
			Register(string, func(*gorm.DB)) error
		}
		after interface {
			Register(string, func(*gorm.DB)) error
		}
	}{
		{"create", callbacks.Create().Before("gorm:create"), callbacks.Create().After("gorm:create")},
		{"query", callbacks.Query().Before("gorm:query"), callbacks.Query().After("gorm:query")},
		{"update", callbacks.Update().Before("gorm:update"), callbacks.Update().After("gorm:update")},
		{"delete", callbacks.Delete().Before("gorm:delete"), callbacks.Delete().After("gorm:delete")},
		{"row", callbacks.Row().Before("gorm:row"), callbacks.Row().After("gorm:row")},
		{"raw", callbacks.Raw().Before("gorm:raw"), callbacks.Raw().After("gorm:raw")},
	}
	for _, registration := range registrations {
		operation := registration.operation
		if err := registration.before.Register("nexaflow:metrics:before:"+operation, func(tx *gorm.DB) {
			tx.InstanceSet("nexaflow:database:started", time.Now())
		}); err != nil {
			return err
		}
		if err := registration.after.Register("nexaflow:metrics:after:"+operation, func(tx *gorm.DB) {
			value, ok := tx.InstanceGet("nexaflow:database:started")
			if !ok {
				return
			}
			started, ok := value.(time.Time)
			if !ok {
				return
			}
			ctx := tx.Statement.Context
			attrs := metric.WithAttributes(attribute.String("db.operation.name", operation))
			p.duration.Record(ctx, time.Since(started).Seconds(), attrs)
			if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				p.errors.Add(ctx, 1, attrs)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}
