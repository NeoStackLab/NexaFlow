package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	minPriority = 0
	maxPriority = 9
)

var ErrHandlerRegistered = errors.New("job handler already registered")

type Job struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Payload        json.RawMessage   `json:"payload,omitempty"`
	Priority       int               `json:"priority"`
	Attempt        int               `json:"attempt"`
	MaxAttempts    int               `json:"max_attempts"`
	AvailableAt    time.Time         `json:"available_at"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Trace          map[string]string `json:"trace,omitempty"`
}

type Request struct {
	Kind           string
	Payload        any
	Priority       int
	MaxAttempts    int
	Delay          time.Duration
	IdempotencyKey string
}

type Handler func(context.Context, Job) error

type Telemetry interface {
	Enqueued(context.Context, string)
	Completed(context.Context, string, time.Duration)
	Retried(context.Context, string)
	DeadLettered(context.Context, string)
	Depth(context.Context, int64, int64)
}

type noopTelemetry struct{}

func (noopTelemetry) Enqueued(context.Context, string)                 {}
func (noopTelemetry) Completed(context.Context, string, time.Duration) {}
func (noopTelemetry) Retried(context.Context, string)                  {}
func (noopTelemetry) DeadLettered(context.Context, string)             {}
func (noopTelemetry) Depth(context.Context, int64, int64)              {}

type schedule struct {
	name    string
	every   time.Duration
	request Request
}

type Queue struct {
	client    redis.UniversalClient
	config    config.JobsConfig
	log       *zap.Logger
	telemetry Telemetry
	handlers  map[string]Handler
	schedules []schedule
	mu        sync.RWMutex
}

func New(client redis.UniversalClient, cfg config.JobsConfig, log *zap.Logger, telemetry Telemetry) *Queue {
	if telemetry == nil {
		telemetry = noopTelemetry{}
	}
	return &Queue{client: client, config: cfg, log: log, telemetry: telemetry, handlers: make(map[string]Handler)}
}

func (q *Queue) Register(kind string, handler Handler) error {
	if kind == "" || handler == nil {
		return errors.New("job kind and handler are required")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.handlers[kind]; exists {
		return fmt.Errorf("%w: %s", ErrHandlerRegistered, kind)
	}
	q.handlers[kind] = handler
	return nil
}

func (q *Queue) Schedule(name string, every time.Duration, request Request) error {
	if name == "" || every <= 0 || request.Kind == "" {
		return errors.New("schedule name, interval, and job kind are required")
	}
	q.schedules = append(q.schedules, schedule{name: name, every: every, request: request})
	return nil
}

func (q *Queue) Enqueue(ctx context.Context, request Request) (Job, error) {
	if request.Kind == "" {
		return Job{}, errors.New("job kind is required")
	}
	if request.Priority < minPriority || request.Priority > maxPriority {
		return Job{}, fmt.Errorf("job priority must be between %d and %d", minPriority, maxPriority)
	}
	if request.MaxAttempts < 1 {
		request.MaxAttempts = 5
	}
	payload, err := json.Marshal(request.Payload)
	if err != nil {
		return Job{}, fmt.Errorf("marshal job payload: %w", err)
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	job := Job{
		ID: newID(), Kind: request.Kind, Payload: payload, Priority: request.Priority,
		MaxAttempts: request.MaxAttempts, AvailableAt: time.Now().UTC().Add(request.Delay),
		IdempotencyKey: request.IdempotencyKey, Trace: map[string]string(carrier),
	}
	encoded, err := json.Marshal(job)
	if err != nil {
		return Job{}, err
	}
	idempotencyKey := ""
	if request.IdempotencyKey != "" {
		idempotencyKey = q.key("idempotency", request.Kind, request.IdempotencyKey)
	}
	result, err := enqueueScript.Run(ctx, q.client, []string{
		q.key("jobs"), q.key("priorities"), q.readyKey(request.Priority), q.key("delayed"), idempotencyKey,
	}, job.ID, string(encoded), request.Priority, job.AvailableAt.UnixMilli(), time.Now().UTC().UnixMilli(), q.config.IdempotencyTTL.Milliseconds()).Text()
	if err != nil {
		return Job{}, fmt.Errorf("enqueue job: %w", err)
	}
	if result != job.ID {
		existing, getErr := q.get(ctx, result)
		if errors.Is(getErr, redis.Nil) {
			return Job{ID: result, Kind: request.Kind, IdempotencyKey: request.IdempotencyKey}, nil
		}
		if getErr != nil {
			return Job{}, getErr
		}
		return existing, nil
	}
	q.telemetry.Enqueued(ctx, job.Kind)
	return job, nil
}

func (q *Queue) ReplayDLQ(ctx context.Context, limit int64) (int64, error) {
	if limit < 1 {
		limit = 100
	}
	ids, err := q.client.ZRange(ctx, q.key("dlq"), 0, limit-1).Result()
	if err != nil {
		return 0, fmt.Errorf("replay dead-letter queue: %w", err)
	}
	var replayed int64
	for _, id := range ids {
		job, getErr := q.get(ctx, id)
		if getErr != nil {
			return replayed, getErr
		}
		job.Attempt = 0
		job.AvailableAt = time.Now().UTC()
		encoded, marshalErr := json.Marshal(job)
		if marshalErr != nil {
			return replayed, marshalErr
		}
		moved, moveErr := replayScript.Run(ctx, q.client, []string{q.key("dlq"), q.key("jobs"), q.readyKey(job.Priority)}, id, string(encoded)).Int64()
		if moveErr != nil {
			return replayed, fmt.Errorf("replay job %s: %w", id, moveErr)
		}
		replayed += moved
	}
	return replayed, nil
}

func (q *Queue) Run(ctx context.Context) error {
	if len(q.handlers) == 0 {
		return errors.New("no job handlers registered")
	}
	var wg sync.WaitGroup
	wg.Add(q.config.Concurrency + 1)
	for worker := 0; worker < q.config.Concurrency; worker++ {
		go func() {
			defer wg.Done()
			q.worker(ctx)
		}()
	}
	go func() {
		defer wg.Done()
		q.maintenance(ctx)
	}()
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (q *Queue) worker(ctx context.Context) {
	ticker := time.NewTicker(q.config.PollInterval)
	defer ticker.Stop()
	for {
		if job, ok := q.claim(ctx); ok {
			q.handle(ctx, job)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (q *Queue) maintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	q.maintain(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.maintain(ctx)
		}
	}
}

func (q *Queue) maintain(ctx context.Context) {
	now := time.Now().UTC()
	args := q.readyArgs()
	args = append(args, now.UnixMilli(), 250)
	if err := promoteScript.Run(ctx, q.client, []string{q.key("delayed"), q.key("priorities")}, args...).Err(); err != nil && !errors.Is(err, context.Canceled) {
		q.log.Warn("job_promotion_failed", zap.Error(err))
	}
	if err := reclaimScript.Run(ctx, q.client, []string{q.key("processing"), q.key("priorities")}, args...).Err(); err != nil && !errors.Is(err, context.Canceled) {
		q.log.Warn("job_reclaim_failed", zap.Error(err))
	}
	for _, item := range q.schedules {
		request := item.request
		request.IdempotencyKey = item.name + ":" + strconv.FormatInt(now.UnixNano()/item.every.Nanoseconds(), 10)
		if _, err := q.Enqueue(ctx, request); err != nil && !errors.Is(err, context.Canceled) {
			q.log.Warn("job_schedule_failed", zap.String("schedule", item.name), zap.Error(err))
		}
	}
	ready, dead, err := q.depth(ctx)
	if err == nil {
		q.telemetry.Depth(ctx, ready, dead)
	}
}

func (q *Queue) claim(ctx context.Context) (Job, bool) {
	keys := append(q.readyKeys(), q.key("processing"))
	id, err := claimScript.Run(ctx, q.client, keys, time.Now().UTC().Add(q.config.LeaseTimeout).UnixMilli()).Text()
	if err != nil {
		if !errors.Is(err, redis.Nil) && !errors.Is(err, context.Canceled) {
			q.log.Warn("job_claim_failed", zap.Error(err))
		}
		return Job{}, false
	}
	job, err := q.get(ctx, id)
	if err != nil {
		q.log.Error("job_load_failed", zap.String("job_id", id), zap.Error(err))
		_ = q.client.ZRem(ctx, q.key("processing"), id).Err()
		return Job{}, false
	}
	return job, true
}

func (q *Queue) handle(parent context.Context, job Job) {
	carrier := propagation.MapCarrier(job.Trace)
	ctx := otel.GetTextMapPropagator().Extract(parent, carrier)
	ctx, span := otel.Tracer("nexaflow/jobs").Start(ctx, job.Kind, trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attribute.String("job.id", job.ID), attribute.Int("job.attempt", job.Attempt)))
	started := time.Now()
	defer span.End()
	stopLease := make(chan struct{})
	defer close(stopLease)
	go q.renewLease(ctx, job.ID, stopLease)
	q.mu.RLock()
	handler := q.handlers[job.Kind]
	q.mu.RUnlock()
	var err error
	if handler == nil {
		err = fmt.Errorf("no handler registered for job kind %q", job.Kind)
	} else {
		err = handler(ctx, job)
	}
	if err == nil {
		if ackErr := ackScript.Run(ctx, q.client, []string{q.key("processing"), q.key("jobs"), q.key("priorities")}, job.ID).Err(); ackErr != nil {
			span.RecordError(ackErr)
			span.SetStatus(codes.Error, ackErr.Error())
			q.log.Error("job_ack_failed", zap.String("job_id", job.ID), zap.Error(ackErr))
			return
		}
		q.telemetry.Completed(ctx, job.Kind, time.Since(started))
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	job.Attempt++
	encoded, marshalErr := json.Marshal(job)
	if marshalErr != nil {
		q.log.Error("job_marshal_failed", zap.String("job_id", job.ID), zap.Error(marshalErr))
		return
	}
	if job.Attempt >= job.MaxAttempts {
		if moveErr := deadScript.Run(ctx, q.client, []string{q.key("processing"), q.key("jobs"), q.key("dlq")}, job.ID, string(encoded), time.Now().UTC().UnixMilli()).Err(); moveErr != nil {
			q.log.Error("job_dead_letter_failed", zap.String("job_id", job.ID), zap.Error(moveErr))
			return
		}
		q.telemetry.DeadLettered(ctx, job.Kind)
		q.log.Error("job_dead_lettered", zap.String("job_id", job.ID), zap.String("kind", job.Kind), zap.Error(err))
		return
	}
	backoff := q.config.RetryBase * time.Duration(math.Pow(2, float64(job.Attempt-1)))
	if backoff > q.config.RetryMax {
		backoff = q.config.RetryMax
	}
	job.AvailableAt = time.Now().UTC().Add(backoff)
	encoded, _ = json.Marshal(job)
	if retryErr := retryScript.Run(ctx, q.client, []string{q.key("processing"), q.key("jobs"), q.key("delayed")}, job.ID, string(encoded), job.AvailableAt.UnixMilli()).Err(); retryErr != nil {
		q.log.Error("job_retry_failed", zap.String("job_id", job.ID), zap.Error(retryErr))
		return
	}
	q.telemetry.Retried(ctx, job.Kind)
}

func (q *Queue) renewLease(ctx context.Context, jobID string, stop <-chan struct{}) {
	interval := q.config.LeaseTimeout / 2
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if err := q.client.ZAddArgs(ctx, q.key("processing"), redis.ZAddArgs{XX: true, Members: []redis.Z{{Score: float64(time.Now().UTC().Add(q.config.LeaseTimeout).UnixMilli()), Member: jobID}}}).Err(); err != nil && !errors.Is(err, context.Canceled) {
				q.log.Warn("job_lease_renewal_failed", zap.String("job_id", jobID), zap.Error(err))
			}
		}
	}
}

func (q *Queue) get(ctx context.Context, id string) (Job, error) {
	encoded, err := q.client.HGet(ctx, q.key("jobs"), id).Bytes()
	if err != nil {
		return Job{}, fmt.Errorf("load job %s: %w", id, err)
	}
	var job Job
	if err := json.Unmarshal(encoded, &job); err != nil {
		return Job{}, fmt.Errorf("decode job %s: %w", id, err)
	}
	return job, nil
}

func (q *Queue) depth(ctx context.Context) (int64, int64, error) {
	pipe := q.client.Pipeline()
	ready := make([]*redis.IntCmd, 0, maxPriority+2)
	for priority := minPriority; priority <= maxPriority; priority++ {
		ready = append(ready, pipe.LLen(ctx, q.readyKey(priority)))
	}
	ready = append(ready, pipe.ZCard(ctx, q.key("delayed")))
	dead := pipe.ZCard(ctx, q.key("dlq"))
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, 0, err
	}
	var total int64
	for _, command := range ready {
		total += command.Val()
	}
	return total, dead.Val(), nil
}

func (q *Queue) key(parts ...string) string {
	key := q.config.Namespace + ":jobs"
	for _, part := range parts {
		key += ":" + part
	}
	return key
}

func (q *Queue) readyKey(priority int) string { return q.key("ready", strconv.Itoa(priority)) }
func (q *Queue) readyKeys() []string {
	keys := make([]string, 0, maxPriority+1)
	for priority := maxPriority; priority >= minPriority; priority-- {
		keys = append(keys, q.readyKey(priority))
	}
	return keys
}

func (q *Queue) readyArgs() []interface{} {
	keys := q.readyKeys()
	args := make([]interface{}, len(keys))
	for index, key := range keys {
		args[index] = key
	}
	return args
}

func newID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

var enqueueScript = redis.NewScript(`
if KEYS[5] ~= '' then
  local existing = redis.call('GET', KEYS[5])
  if existing then return existing end
  redis.call('SET', KEYS[5], ARGV[1], 'PX', ARGV[6], 'NX')
end
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('HSET', KEYS[2], ARGV[1], ARGV[3])
if tonumber(ARGV[4]) > tonumber(ARGV[5]) then
  redis.call('ZADD', KEYS[4], ARGV[4], ARGV[1])
else
  redis.call('RPUSH', KEYS[3], ARGV[1])
end
return ARGV[1]
`)

var claimScript = redis.NewScript(`
for i = 1, #KEYS - 1 do
  local id = redis.call('LPOP', KEYS[i])
  if id then
    redis.call('ZADD', KEYS[#KEYS], ARGV[1], id)
    return id
  end
end
return nil
`)

var promoteScript = redis.NewScript(`
local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[11], 'LIMIT', 0, ARGV[12])
for _, id in ipairs(ids) do
  local priority = redis.call('HGET', KEYS[2], id)
  if priority then
    redis.call('RPUSH', ARGV[10 - tonumber(priority)], id)
  end
  redis.call('ZREM', KEYS[1], id)
end
return #ids
`)

var reclaimScript = promoteScript

var ackScript = redis.NewScript(`
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
redis.call('HDEL', KEYS[3], ARGV[1])
return 1
`)

var retryScript = redis.NewScript(`
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
redis.call('ZADD', KEYS[3], ARGV[3], ARGV[1])
return 1
`)

var deadScript = redis.NewScript(`
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
redis.call('ZADD', KEYS[3], ARGV[3], ARGV[1])
return 1
`)

var replayScript = redis.NewScript(`
if redis.call('ZREM', KEYS[1], ARGV[1]) == 0 then return 0 end
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
redis.call('RPUSH', KEYS[3], ARGV[1])
return 1
`)
