package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
)

var (
	ErrRecordNotFound = errors.New("dynamic record not found")
	ErrRecordConflict = errors.New("dynamic record version conflict")
)

type DynamicRecordRepository struct{ install InstallRepository }

func NewDynamicRecordRepository(install InstallRepository) *DynamicRecordRepository {
	return &DynamicRecordRepository{install: install}
}

func (r *DynamicRecordRepository) withDB(ctx context.Context, fn func(*gorm.DB) error) error {
	return withInstallDB(ctx, r.install, fn)
}

func (r *DynamicRecordRepository) Create(ctx context.Context, tenantID, entityID, actorID string, values map[string]any) (model.RecordView, error) {
	var result model.RecordView
	err := r.withDB(ctx, func(db *gorm.DB) error {
		encoded, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("encode record values: %w", err)
		}
		now := time.Now().UTC()
		record := model.DynamicRecord{ID: newUUID(), TenantID: tenantID, EntityID: entityID, Values: encoded, Version: 1, Status: "active", CreatedBy: actorID, UpdatedBy: actorID, CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&record).Error; err != nil {
			return err
		}
		result = assembleRecord(record)
		return nil
	})
	return result, err
}

func (r *DynamicRecordRepository) List(ctx context.Context, tenantID, entityID string, offset, limit int) ([]model.RecordView, int64, error) {
	return r.ListWithQuery(ctx, tenantID, entityID, "", offset, limit)
}

func (r *DynamicRecordRepository) ListWithQuery(ctx context.Context, tenantID, entityID, search string, offset, limit int) ([]model.RecordView, int64, error) {
	result := []model.RecordView{}
	var total int64
	err := r.withDB(ctx, func(db *gorm.DB) error {
		search = strings.TrimSpace(search)
		query := db.Model(&model.DynamicRecord{}).Where("tenant_id = ? AND entity_id = ? AND status = ?", tenantID, entityID, "active")
		if search != "" {
			query = query.Where(`"values"::text ILIKE ? ESCAPE '!'`, "%"+escapeLike(search)+"%")
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		var records []model.DynamicRecord
		query = db.Where("tenant_id = ? AND entity_id = ? AND status = ?", tenantID, entityID, "active")
		if search != "" {
			query = query.Where(`"values"::text ILIKE ? ESCAPE '!'`, "%"+escapeLike(search)+"%")
		}
		if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			result = append(result, assembleRecord(record))
		}
		return nil
	})
	return result, total, err
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	return strings.ReplaceAll(value, "_", "!_")
}

func (r *DynamicRecordRepository) Get(ctx context.Context, tenantID, entityID, recordID string) (model.RecordView, error) {
	var result model.RecordView
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var record model.DynamicRecord
		if err := db.Where("tenant_id = ? AND entity_id = ? AND id = ? AND status = ?", tenantID, entityID, recordID, "active").First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRecordNotFound
			}
			return err
		}
		result = assembleRecord(record)
		return nil
	})
	return result, err
}

func (r *DynamicRecordRepository) Update(ctx context.Context, tenantID, entityID, recordID, actorID string, expectedVersion int, values map[string]any) (model.RecordView, error) {
	var result model.RecordView
	err := r.withDB(ctx, func(db *gorm.DB) error {
		encoded, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("encode record values: %w", err)
		}
		update := db.Model(&model.DynamicRecord{}).Where("tenant_id = ? AND entity_id = ? AND id = ? AND status = ? AND version = ?", tenantID, entityID, recordID, "active", expectedVersion).Updates(map[string]any{"values": encoded, "updated_by": actorID, "updated_at": time.Now().UTC(), "version": gorm.Expr("version + 1")})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return r.classifyMissingRecord(db, tenantID, entityID, recordID)
		}
		var record model.DynamicRecord
		if err := db.Where("tenant_id = ? AND entity_id = ? AND id = ? AND status = ?", tenantID, entityID, recordID, "active").First(&record).Error; err != nil {
			return err
		}
		result = assembleRecord(record)
		return nil
	})
	return result, err
}

func (r *DynamicRecordRepository) Delete(ctx context.Context, tenantID, entityID, recordID string, expectedVersion int) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		update := db.Model(&model.DynamicRecord{}).Where("tenant_id = ? AND entity_id = ? AND id = ? AND status = ? AND version = ?", tenantID, entityID, recordID, "active", expectedVersion).Updates(map[string]any{"status": "archived", "updated_at": time.Now().UTC(), "version": gorm.Expr("version + 1")})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return r.classifyMissingRecord(db, tenantID, entityID, recordID)
		}
		return nil
	})
}

func (r *DynamicRecordRepository) classifyMissingRecord(db *gorm.DB, tenantID, entityID, recordID string) error {
	var count int64
	if err := db.Model(&model.DynamicRecord{}).Where("tenant_id = ? AND entity_id = ? AND id = ? AND status = ?", tenantID, entityID, recordID, "active").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrRecordNotFound
	}
	return ErrRecordConflict
}

func assembleRecord(record model.DynamicRecord) model.RecordView {
	values := map[string]any{}
	_ = json.Unmarshal(record.Values, &values)
	return model.RecordView{ID: record.ID, EntityID: record.EntityID, Values: values, Version: record.Version, CreatedBy: record.CreatedBy, UpdatedBy: record.UpdatedBy, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
