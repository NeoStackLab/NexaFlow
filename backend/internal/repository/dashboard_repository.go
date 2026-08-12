package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
)

var ErrDashboardConflict = errors.New("dashboard version conflict")

type DashboardRepository struct{ knowledge *KnowledgeRepository }

func NewDashboardRepository(install InstallRepository) *DashboardRepository {
	return &DashboardRepository{knowledge: NewKnowledgeRepository(install)}
}

func (r *DashboardRepository) Get(ctx context.Context, tenantID string) (model.DashboardView, error) {
	var result model.DashboardView
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		var dashboard model.Dashboard
		err := db.Where("tenant_id = ?", tenantID).First(&dashboard).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = model.DashboardView{Widgets: []model.DashboardWidget{{ID: "users", Type: "users", Title: "活跃用户", Width: 1}, {ID: "files", Type: "files", Title: "文件数量", Width: 1}}, Values: map[string]float64{}, Version: 0}
			return nil
		}
		if err != nil {
			return err
		}
		result.Version = dashboard.Version
		return json.Unmarshal(dashboard.Widgets, &result.Widgets)
	})
	return result, err
}

func (r *DashboardRepository) Save(ctx context.Context, tenantID, actorID string, input model.SaveDashboardInput) (model.DashboardView, error) {
	var result model.DashboardView
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			payload, err := json.Marshal(input.Widgets)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			var dashboard model.Dashboard
			err = tx.Where("tenant_id = ?", tenantID).First(&dashboard).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if input.ExpectedVersion != 0 {
					return ErrDashboardConflict
				}
				dashboard = model.Dashboard{ID: newUUID(), TenantID: tenantID, Widgets: payload, Version: 1, UpdatedBy: actorID, CreatedAt: now, UpdatedAt: now}
				if err = tx.Create(&dashboard).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				update := tx.Model(&dashboard).Where("version = ?", input.ExpectedVersion).Updates(map[string]any{"widgets": payload, "version": gorm.Expr("version + 1"), "updated_by": actorID, "updated_at": now})
				if update.Error != nil {
					return update.Error
				}
				if update.RowsAffected == 0 {
					return ErrDashboardConflict
				}
				dashboard.Version++
			}
			result = model.DashboardView{Widgets: input.Widgets, Values: map[string]float64{}, Version: dashboard.Version}
			return nil
		})
	})
	return result, err
}

func (r *DashboardRepository) Values(ctx context.Context, tenantID string, widgets []model.DashboardWidget) (map[string]float64, error) {
	result := map[string]float64{}
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		for _, widget := range widgets {
			var value float64
			var err error
			switch widget.Type {
			case "users":
				var count int64
				err = db.Model(&model.TenantMembership{}).Where("tenant_id = ? AND status = ?", tenantID, "active").Count(&count).Error
				value = float64(count)
			case "files":
				var count int64
				err = db.Model(&model.FileAsset{}).Where("tenant_id = ? AND status = ?", tenantID, "active").Count(&count).Error
				value = float64(count)
			case "records":
				var count int64
				err = db.Model(&model.DynamicRecord{}).Where("tenant_id = ? AND entity_id = ? AND status = ?", tenantID, widget.EntityID, "active").Count(&count).Error
				value = float64(count)
			case "sum":
				err = db.Raw(`SELECT COALESCE(SUM((values ->> ?)::numeric), 0) FROM dynamic_records WHERE tenant_id = ? AND entity_id = ? AND status = 'active'`, widget.Field, tenantID, widget.EntityID).Scan(&value).Error
			}
			if err != nil {
				return err
			}
			result[widget.ID] = value
		}
		return nil
	})
	return result, err
}
