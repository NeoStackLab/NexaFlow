package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
)

var ErrFormSubmissionNotFound = errors.New("form submission not found")

type FormSubmissionRepository interface {
	Create(ctx context.Context, tenantID, formID, entityID, recordID, submittedBy string, values map[string]any) (model.FormSubmission, error)
	List(ctx context.Context, tenantID, formID string, offset, limit int) ([]model.FormSubmission, int64, error)
}

type formSubmissionRepository struct{ install InstallRepository }

func NewFormSubmissionRepository(install InstallRepository) FormSubmissionRepository {
	return &formSubmissionRepository{install: install}
}

func (r *formSubmissionRepository) Create(ctx context.Context, tenantID, formID, entityID, recordID, submittedBy string, values map[string]any) (model.FormSubmission, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return model.FormSubmission{}, err
	}
	result := model.FormSubmission{}
	err = withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		now := time.Now().UTC()
		row := model.FormSubmission{ID: newUUID(), TenantID: tenantID, FormID: formID, EntityID: entityID, RecordID: recordID, SubmittedBy: submittedBy, Values: encoded, Status: "accepted", CreatedAt: now}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
		result = row
		return nil
	})
	return result, err
}

func (r *formSubmissionRepository) List(ctx context.Context, tenantID, formID string, offset, limit int) ([]model.FormSubmission, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	result := []model.FormSubmission{}
	var total int64
	err := withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		query := db.Model(&model.FormSubmission{}).Where("tenant_id = ? AND form_id = ?", tenantID, formID)
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&result).Error
	})
	return result, total, err
}
