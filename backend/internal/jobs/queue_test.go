package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func testQueue(t *testing.T) (*Queue, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	queue := New(client, config.JobsConfig{
		Namespace: "test", Concurrency: 1, PollInterval: time.Millisecond,
		LeaseTimeout: time.Minute, RetryBase: time.Millisecond, RetryMax: time.Second,
		IdempotencyTTL: time.Hour,
	}, zap.NewNop(), nil)
	return queue, server
}

func TestQueueHonorsPriorityAndIdempotency(t *testing.T) {
	queue, _ := testQueue(t)
	ctx := context.Background()
	low, err := queue.Enqueue(ctx, Request{Kind: "test", Priority: 1, IdempotencyKey: "low"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := queue.Enqueue(ctx, Request{Kind: "test", Priority: 1, IdempotencyKey: "low"})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != low.ID {
		t.Fatalf("duplicate ID = %q, want %q", duplicate.ID, low.ID)
	}
	high, err := queue.Enqueue(ctx, Request{Kind: "test", Priority: 9})
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := queue.claim(ctx)
	if !ok || claimed.ID != high.ID {
		t.Fatalf("claimed = %#v, want high-priority %q", claimed, high.ID)
	}
}

func TestCompletedIdempotentJobIsNotEnqueuedAgain(t *testing.T) {
	queue, _ := testQueue(t)
	ctx := context.Background()
	if err := queue.Register("once", func(context.Context, Job) error { return nil }); err != nil {
		t.Fatal(err)
	}
	job, err := queue.Enqueue(ctx, Request{Kind: "once", Priority: 5, IdempotencyKey: "business-key"})
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := queue.claim(ctx)
	if !ok {
		t.Fatal("job was not claimed")
	}
	queue.handle(ctx, claimed)
	duplicate, err := queue.Enqueue(ctx, Request{Kind: "once", Priority: 5, IdempotencyKey: "business-key"})
	if err != nil || duplicate.ID != job.ID {
		t.Fatalf("duplicate = %#v, %v; want completed ID %s", duplicate, err, job.ID)
	}
	if _, ok := queue.claim(ctx); ok {
		t.Fatal("completed idempotent job was enqueued again")
	}
}

func TestQueueRetriesToDLQAndReplays(t *testing.T) {
	queue, _ := testQueue(t)
	ctx := context.Background()
	fail := true
	if err := queue.Register("test", func(context.Context, Job) error {
		if fail {
			return errors.New("boom")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	job, err := queue.Enqueue(ctx, Request{Kind: "test", Priority: 5, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := queue.claim(ctx)
	if !ok {
		t.Fatal("job was not claimed")
	}
	queue.handle(ctx, claimed)
	if got, _ := queue.client.ZRange(ctx, queue.key("dlq"), 0, -1).Result(); len(got) != 1 || got[0] != job.ID {
		t.Fatalf("DLQ = %#v, want %q", got, job.ID)
	}
	fail = false
	if count, err := queue.ReplayDLQ(ctx, 10); err != nil || count != 1 {
		t.Fatalf("ReplayDLQ() = %d, %v", count, err)
	}
	replayed, ok := queue.claim(ctx)
	if !ok {
		t.Fatal("replayed job was not claimed")
	}
	queue.handle(ctx, replayed)
	if queue.client.HExists(ctx, queue.key("jobs"), job.ID).Val() {
		t.Fatal("completed replayed job was not acknowledged")
	}
}

func TestQueueRetriesFailedWorkAfterBackoff(t *testing.T) {
	queue, _ := testQueue(t)
	queue.config.RetryBase = time.Nanosecond
	ctx := context.Background()
	attempts := 0
	if err := queue.Register("retry", func(context.Context, Job) error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	job, err := queue.Enqueue(ctx, Request{Kind: "retry", Priority: 4, MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := queue.claim(ctx)
	if !ok {
		t.Fatal("job was not claimed")
	}
	queue.handle(ctx, claimed)
	queue.maintain(ctx)
	retry, ok := queue.claim(ctx)
	if !ok || retry.ID != job.ID || retry.Attempt != 1 {
		t.Fatalf("retry = %#v, want attempt 1 for %s", retry, job.ID)
	}
	queue.handle(ctx, retry)
	if attempts != 2 || queue.client.HExists(ctx, queue.key("jobs"), job.ID).Val() {
		t.Fatalf("attempts = %d; completed job still exists", attempts)
	}
}
