package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"go.uber.org/zap"
)

const maxKnowledgeUploadSize int64 = 20 << 20

type KnowledgeIngestionService interface {
	Enqueue(context.Context, string, string, string, string, io.Reader) (model.KnowledgeIngestionJob, error)
	List(context.Context, string, int) ([]model.KnowledgeIngestionJob, error)
}

type knowledgeIngestionService struct {
	repository repository.KnowledgeIngestionRepository
	store      ObjectStore
}

func NewKnowledgeIngestionService(repo repository.KnowledgeIngestionRepository, store ObjectStore) KnowledgeIngestionService {
	return &knowledgeIngestionService{repository: repo, store: store}
}

func (s *knowledgeIngestionService) Enqueue(ctx context.Context, tenantID, actorID, name, contentType string, reader io.Reader) (model.KnowledgeIngestionJob, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" || len(name) > 255 {
		return model.KnowledgeIngestionJob{}, fmt.Errorf("%w: invalid file name", ErrInvalidKnowledgeDocument)
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxKnowledgeUploadSize+1))
	if err != nil {
		return model.KnowledgeIngestionJob{}, fmt.Errorf("%w: read upload failed", ErrInvalidKnowledgeDocument)
	}
	if len(data) < 1 || int64(len(data)) > maxKnowledgeUploadSize {
		return model.KnowledgeIngestionJob{}, fmt.Errorf("%w: file must be 1 byte to 20 MiB", ErrInvalidKnowledgeDocument)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	jobID, err := newIngestionUUID()
	if err != nil {
		return model.KnowledgeIngestionJob{}, err
	}
	job := model.KnowledgeIngestionJob{ID: jobID, TenantID: tenantID, CreatedBy: actorID, Name: name, ContentType: contentType, Size: int64(len(data)), StorageKey: "knowledge-ingestion/" + tenantID + "/" + jobID + "/payload", Status: "pending"}
	if err := s.store.Put(ctx, job.StorageKey, bytes.NewReader(data), job.Size, job.ContentType); err != nil {
		return model.KnowledgeIngestionJob{}, fmt.Errorf("store knowledge upload: %w", err)
	}
	persisted, err := s.repository.Create(ctx, job)
	if err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), job.StorageKey)
		return model.KnowledgeIngestionJob{}, err
	}
	return persisted, nil
}

func (s *knowledgeIngestionService) List(ctx context.Context, tenantID string, limit int) ([]model.KnowledgeIngestionJob, error) {
	return s.repository.List(ctx, tenantID, limit)
}

type KnowledgeIngestionWorker struct {
	repository repository.KnowledgeIngestionRepository
	store      ObjectStore
	knowledge  KnowledgeService
	config     config.KnowledgeConfig
	log        *zap.Logger
}

func NewKnowledgeIngestionWorker(repo repository.KnowledgeIngestionRepository, store ObjectStore, knowledge KnowledgeService, cfg config.KnowledgeConfig, log *zap.Logger) *KnowledgeIngestionWorker {
	return &KnowledgeIngestionWorker{repository: repo, store: store, knowledge: knowledge, config: cfg, log: log}
}

func (w *KnowledgeIngestionWorker) ProcessBatch(ctx context.Context) error {
	limit := w.config.BatchSize
	if limit < 1 {
		limit = 5
	}
	jobs, err := w.repository.ClaimPending(tenantcontext.WithSystem(ctx), limit, 10*time.Minute)
	if err != nil {
		w.log.Error("knowledge_ingestion_claim_failed", zap.Error(err))
		return err
	}
	for _, job := range jobs {
		if err := w.processJob(ctx, job); err != nil {
			w.log.Warn("knowledge_ingestion_failed", zap.String("job_id", job.ID), zap.Error(err))
		}
	}
	return nil
}

func (w *KnowledgeIngestionWorker) process(ctx context.Context) { _ = w.ProcessBatch(ctx) }

func (w *KnowledgeIngestionWorker) processJob(ctx context.Context, job model.KnowledgeIngestionJob) error {
	ctx = tenantcontext.With(ctx, tenantcontext.Scope{TenantID: job.TenantID, UserID: job.CreatedBy})
	source, err := w.store.Open(ctx, job.StorageKey)
	if err == nil {
		var document model.KnowledgeDocument
		document, err = w.knowledge.Ingest(ctx, job.TenantID, job.CreatedBy, job.Name, job.ContentType, job.Size, source)
		_ = source.Close()
		if err == nil {
			err = w.repository.MarkCompleted(ctx, job.ID, document.ID)
			if err == nil {
				_ = w.store.Delete(context.WithoutCancel(ctx), job.StorageKey)
			}
			return err
		}
	}
	reason := truncateIngestionError(err)
	maxAttempts := w.config.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	if job.AttemptCount >= maxAttempts {
		markErr := w.repository.MarkDeadLetter(ctx, job.ID, reason)
		if markErr == nil {
			_ = w.store.Delete(context.WithoutCancel(ctx), job.StorageKey)
		}
		return markErrOrOriginal(markErr, err)
	}
	return markErrOrOriginal(w.repository.MarkFailed(ctx, job.ID, reason, nextKnowledgeAttempt(job.AttemptCount)), err)
}

func nextKnowledgeAttempt(attempt int) *time.Time {
	if attempt < 1 {
		attempt = 1
	}
	next := time.Now().UTC().Add(time.Duration(1<<min(attempt-1, 5)) * time.Minute)
	return &next
}

func markErrOrOriginal(markErr, original error) error {
	if markErr != nil {
		return markErr
	}
	return original
}

func truncateIngestionError(err error) string {
	if err == nil {
		return "knowledge ingestion failed"
	}
	message := err.Error()
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func newIngestionUUID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
