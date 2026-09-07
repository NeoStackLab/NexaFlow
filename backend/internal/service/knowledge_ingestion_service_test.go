package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"go.uber.org/zap"
)

type ingestionRepositoryStub struct {
	job       model.KnowledgeIngestionJob
	completed string
}

func (s *ingestionRepositoryStub) Create(_ context.Context, job model.KnowledgeIngestionJob) (model.KnowledgeIngestionJob, error) {
	s.job = job
	return job, nil
}
func (s *ingestionRepositoryStub) List(context.Context, string, int) ([]model.KnowledgeIngestionJob, error) {
	return []model.KnowledgeIngestionJob{s.job}, nil
}
func (s *ingestionRepositoryStub) ClaimPending(context.Context, int, time.Duration) ([]model.KnowledgeIngestionJob, error) {
	return nil, nil
}
func (s *ingestionRepositoryStub) MarkCompleted(_ context.Context, id, documentID string) error {
	s.completed = id + ":" + documentID
	return nil
}
func (s *ingestionRepositoryStub) MarkFailed(context.Context, string, string, *time.Time) error {
	return nil
}
func (s *ingestionRepositoryStub) MarkDeadLetter(context.Context, string, string) error { return nil }

type ingestionStoreStub struct {
	data    []byte
	deleted string
}

func (s *ingestionStoreStub) Put(_ context.Context, key string, reader io.Reader, size int64, _ string) error {
	s.data, _ = io.ReadAll(reader)
	if int64(len(s.data)) != size {
		return io.ErrShortBuffer
	}
	_ = key
	return nil
}
func (s *ingestionStoreStub) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(s.data))), nil
}
func (s *ingestionStoreStub) Delete(_ context.Context, key string) error { s.deleted = key; return nil }
func (s *ingestionStoreStub) Provider() string                           { return "test" }

type ingestionKnowledgeStub struct{}

func (ingestionKnowledgeStub) Ingest(context.Context, string, string, string, string, int64, io.Reader) (model.KnowledgeDocument, error) {
	return model.KnowledgeDocument{ID: "document-1"}, nil
}
func (ingestionKnowledgeStub) List(context.Context, string) ([]model.KnowledgeDocument, error) {
	return nil, nil
}
func (ingestionKnowledgeStub) Delete(context.Context, string, string) error { return nil }
func (ingestionKnowledgeStub) Search(context.Context, string, string, int) ([]model.KnowledgeSource, error) {
	return nil, nil
}

func TestKnowledgeIngestionEnqueuePersistsObjectBeforeJob(t *testing.T) {
	repo := &ingestionRepositoryStub{}
	store := &ingestionStoreStub{}
	service := NewKnowledgeIngestionService(repo, store)
	job, err := service.Enqueue(context.Background(), "tenant-1", "actor-1", "policy.txt", "text/plain", strings.NewReader("a sufficiently long knowledge payload"))
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "pending" || repo.job.ID != job.ID || len(store.data) == 0 || !strings.HasPrefix(job.StorageKey, "knowledge-ingestion/tenant-1/") {
		t.Fatalf("unexpected queued job: %+v", job)
	}
}

func TestKnowledgeIngestionWorkerCompletesAndCleansObject(t *testing.T) {
	repo := &ingestionRepositoryStub{}
	store := &ingestionStoreStub{data: []byte("worker payload")}
	worker := NewKnowledgeIngestionWorker(repo, store, ingestionKnowledgeStub{}, config.KnowledgeConfig{MaxAttempts: 3}, zap.NewNop())
	err := worker.processJob(context.Background(), model.KnowledgeIngestionJob{ID: "job-1", TenantID: "tenant-1", CreatedBy: "actor-1", Name: "policy.txt", ContentType: "text/plain", Size: int64(len(store.data)), StorageKey: "key-1", AttemptCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repo.completed != "job-1:document-1" || store.deleted != "key-1" {
		t.Fatalf("job was not completed and cleaned: completed=%q deleted=%q", repo.completed, store.deleted)
	}
}

func TestNextKnowledgeAttemptUsesExponentialBackoff(t *testing.T) {
	started := time.Now().UTC()
	next := nextKnowledgeAttempt(3)
	if !next.After(started.Add(3 * time.Minute)) {
		t.Fatalf("expected backoff after three attempts, got %v", next.Sub(started))
	}
}
