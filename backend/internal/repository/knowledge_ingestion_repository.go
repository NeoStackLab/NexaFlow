package repository

import (
	"context"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrKnowledgeIngestionJobNotFound = errors.New("knowledge ingestion job not found")

type KnowledgeIngestionRepository interface {
	Create(context.Context, model.KnowledgeIngestionJob) (model.KnowledgeIngestionJob, error)
	List(context.Context, string, int) ([]model.KnowledgeIngestionJob, error)
	ClaimPending(context.Context, int, time.Duration) ([]model.KnowledgeIngestionJob, error)
	MarkCompleted(context.Context, string, string) error
	MarkFailed(context.Context, string, string, *time.Time) error
	MarkDeadLetter(context.Context, string, string) error
}

type knowledgeIngestionRepository struct{ install InstallRepository }

func NewKnowledgeIngestionRepository(install InstallRepository) KnowledgeIngestionRepository {
	return &knowledgeIngestionRepository{install: install}
}

func (r *knowledgeIngestionRepository) Create(ctx context.Context, job model.KnowledgeIngestionJob) (model.KnowledgeIngestionJob, error) {
	if job.ID == "" {
		job.ID = newUUID()
	}
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	if job.Status == "" {
		job.Status = "pending"
	}
	err := withInstallDB(ctx, r.install, func(db *gorm.DB) error { return db.Create(&job).Error })
	return job, err
}

func (r *knowledgeIngestionRepository) List(ctx context.Context, tenantID string, limit int) ([]model.KnowledgeIngestionJob, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	result := make([]model.KnowledgeIngestionJob, 0, limit)
	err := withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		return db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Limit(limit).Find(&result).Error
	})
	return result, err
}

func (r *knowledgeIngestionRepository) ClaimPending(ctx context.Context, limit int, staleAfter time.Duration) ([]model.KnowledgeIngestionJob, error) {
	if limit < 1 {
		limit = 5
	}
	now := time.Now().UTC()
	staleBefore := now.Add(-staleAfter)
	claimed := make([]model.KnowledgeIngestionJob, 0, limit)
	err := withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var rows []model.KnowledgeIngestionJob
			query := tx.Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND claimed_at < ?)", []string{"pending", "failed"}, now, "processing", staleBefore).
				Order("created_at ASC").Limit(limit).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
			if err := query.Find(&rows).Error; err != nil {
				return err
			}
			for index := range rows {
				row := &rows[index]
				if err := tx.Model(&model.KnowledgeIngestionJob{}).Where("id = ?", row.ID).Updates(map[string]any{"status": "processing", "attempt_count": gorm.Expr("attempt_count + 1"), "claimed_at": now, "updated_at": now, "error": ""}).Error; err != nil {
					return err
				}
				row.Status = "processing"
				row.AttemptCount++
				row.ClaimedAt = &now
				row.Error = ""
				claimed = append(claimed, *row)
			}
			return nil
		})
	})
	return claimed, err
}

func (r *knowledgeIngestionRepository) MarkCompleted(ctx context.Context, id, documentID string) error {
	return r.updateStatus(ctx, id, map[string]any{"status": "completed", "document_id": documentID, "claimed_at": nil, "next_attempt_at": nil, "error": "", "updated_at": time.Now().UTC()})
}

func (r *knowledgeIngestionRepository) MarkFailed(ctx context.Context, id, reason string, nextAttemptAt *time.Time) error {
	return r.updateStatus(ctx, id, map[string]any{"status": "failed", "error": truncateJobError(reason), "next_attempt_at": nextAttemptAt, "claimed_at": nil, "updated_at": time.Now().UTC()})
}

func (r *knowledgeIngestionRepository) MarkDeadLetter(ctx context.Context, id, reason string) error {
	return r.updateStatus(ctx, id, map[string]any{"status": "dead_letter", "error": truncateJobError(reason), "next_attempt_at": nil, "claimed_at": nil, "updated_at": time.Now().UTC()})
}

func (r *knowledgeIngestionRepository) updateStatus(ctx context.Context, id string, updates map[string]any) error {
	return withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		result := db.Model(&model.KnowledgeIngestionJob{}).Where("id = ? AND status = ?", id, "processing").Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrKnowledgeIngestionJobNotFound
		}
		return nil
	})
}

func truncateJobError(reason string) string {
	if len(reason) > 1000 {
		return reason[:1000]
	}
	return reason
}
