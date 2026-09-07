package observability

import (
	"context"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
)

func TestDisabledRuntimeProvidesNoopSafeSignals(t *testing.T) {
	ctx := context.Background()
	runtime, err := Start(ctx, config.ObservabilityConfig{SampleRatio: 1, MetricInterval: time.Second}, "test", "dev", "test")
	if err != nil {
		t.Fatal(err)
	}
	runtime.Metrics.Enqueued(ctx, "test")
	runtime.Metrics.Completed(ctx, "test", time.Millisecond)
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
