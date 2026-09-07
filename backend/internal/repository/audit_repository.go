package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
)

type AuditRepository interface {
	Create(context.Context, model.AuditEvent) error
	List(context.Context, string, int) ([]model.AuditEvent, error)
}

type auditRepository struct{ install InstallRepository }

func NewAuditRepository(install InstallRepository) AuditRepository {
	return &auditRepository{install: install}
}

func (r *auditRepository) Create(ctx context.Context, event model.AuditEvent) error {
	if len(event.Metadata) == 0 {
		event.Metadata = json.RawMessage(`{}`)
	}
	if event.ID == "" {
		event.ID = newUUID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	return withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		return db.Create(&event).Error
	})
}

func (r *auditRepository) List(ctx context.Context, tenantID string, limit int) ([]model.AuditEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	result := make([]model.AuditEvent, 0, limit)
	err := withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		return db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Limit(limit).Find(&result).Error
	})
	return result, err
}
