package repository

import (
	"context"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotificationNotFound = errors.New("notification not found")

type NotificationOutboxRepository interface {
	ClaimPending(ctx context.Context, limit int, staleAfter time.Duration) ([]model.Notification, error)
	MarkDelivered(ctx context.Context, id string, deliveredAt time.Time) error
	MarkFailed(ctx context.Context, id, reason string, nextAttemptAt *time.Time) error
	MarkDeadLetter(ctx context.Context, id, reason string) error
}

type notificationRepository struct{ install InstallRepository }

func NewNotificationRepository(install InstallRepository) NotificationOutboxRepository {
	return &notificationRepository{install: install}
}

func (r *notificationRepository) ClaimPending(ctx context.Context, limit int, staleAfter time.Duration) ([]model.Notification, error) {
	if limit < 1 {
		limit = 25
	}
	now := time.Now().UTC()
	staleBefore := now.Add(-staleAfter)
	claimed := make([]model.Notification, 0, limit)
	err := withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var rows []model.Notification
			query := tx.Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND claimed_at < ?)", []string{"pending", "failed"}, now, "processing", staleBefore).
				Order("created_at ASC").Limit(limit).
				Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
			if err := query.Find(&rows).Error; err != nil {
				return err
			}
			for index := range rows {
				row := &rows[index]
				updates := map[string]any{
					"status":        "processing",
					"attempt_count": gorm.Expr("attempt_count + 1"),
					"claimed_at":    now,
					"last_error":    "",
				}
				if err := tx.Model(&model.Notification{}).Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Updates(updates).Error; err != nil {
					return err
				}
				row.Status = "processing"
				row.AttemptCount++
				row.ClaimedAt = &now
				row.LastError = ""
				claimed = append(claimed, *row)
			}
			return nil
		})
	})
	return claimed, err
}

func (r *notificationRepository) MarkDelivered(ctx context.Context, id string, deliveredAt time.Time) error {
	return withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		result := db.Model(&model.Notification{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
			"status": "delivered", "delivered_at": deliveredAt, "claimed_at": nil, "next_attempt_at": nil, "last_error": "",
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotificationNotFound
		}
		return nil
	})
}

func (r *notificationRepository) MarkFailed(ctx context.Context, id, reason string, nextAttemptAt *time.Time) error {
	return withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		result := db.Model(&model.Notification{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
			"status": "failed", "last_error": reason, "next_attempt_at": nextAttemptAt, "claimed_at": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotificationNotFound
		}
		return nil
	})
}

func (r *notificationRepository) MarkDeadLetter(ctx context.Context, id, reason string) error {
	return withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		result := db.Model(&model.Notification{}).Where("id = ? AND status = ?", id, "processing").Updates(map[string]any{
			"status": "dead_letter", "last_error": reason, "next_attempt_at": nil, "claimed_at": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotificationNotFound
		}
		return nil
	})
}
