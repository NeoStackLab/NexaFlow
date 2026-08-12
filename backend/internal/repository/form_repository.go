package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrFormNotFound = errors.New("form not found")
	ErrFormConflict = errors.New("form version conflict")
	ErrFormSlugUsed = errors.New("form slug already exists")
)

type FormRepository struct{ install InstallRepository }

func NewFormRepository(install InstallRepository) *FormRepository {
	return &FormRepository{install: install}
}

func (r *FormRepository) withDB(ctx context.Context, fn func(*gorm.DB) error) error {
	runtimeConfig, err := r.install.RuntimeConfig()
	if err != nil {
		return fmt.Errorf("load installed database configuration: %w", err)
	}
	db, err := database.Open(ctx, config.DatabaseConfig{Host: runtimeConfig.Database.Host, Port: runtimeConfig.Database.Port, User: runtimeConfig.Database.User, Password: runtimeConfig.Database.Password, Name: runtimeConfig.Database.Name, SSLMode: runtimeConfig.Database.SSLMode, MaxOpenConnections: 5, MaxIdleConnections: 1, ConnectionMaxLifetime: 5 * time.Minute})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return fn(db.WithContext(ctx))
}

func (r *FormRepository) Define(ctx context.Context, tenantID string, input model.DefineFormInput, jsonSchema map[string]any) (model.FormDefinition, error) {
	var result model.FormDefinition
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			schemaBytes, err := json.Marshal(jsonSchema)
			if err != nil {
				return err
			}
			layoutBytes, err := json.Marshal(input.Components)
			if err != nil {
				return err
			}
			form := model.Form{ID: input.ID, TenantID: tenantID, EntityID: input.EntityID, Name: input.Name, Slug: input.Slug, Description: input.Description, JSONSchema: schemaBytes, Layout: layoutBytes, Status: "active", UpdatedAt: now}
			if input.ID == "" {
				var count int64
				if err := tx.Model(&model.Form{}).Where("tenant_id = ? AND slug = ?", tenantID, input.Slug).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return ErrFormSlugUsed
				}
				form.ID, form.Version, form.CreatedAt = newUUID(), 1, now
				if err := tx.Create(&form).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, input.ID, "active").First(&form).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return ErrFormNotFound
					}
					return err
				}
				if form.Version != input.ExpectedVersion {
					return ErrFormConflict
				}
				var count int64
				if err := tx.Model(&model.Form{}).Where("tenant_id = ? AND slug = ? AND id <> ?", tenantID, input.Slug, input.ID).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return ErrFormSlugUsed
				}
				form.EntityID, form.Name, form.Slug, form.Description, form.JSONSchema, form.Layout, form.Version, form.UpdatedAt = input.EntityID, input.Name, input.Slug, input.Description, schemaBytes, layoutBytes, form.Version+1, now
				if err := tx.Model(&form).Select("entity_id", "name", "slug", "description", "json_schema", "layout", "version", "updated_at").Updates(&form).Error; err != nil {
					return err
				}
			}
			result = assembleForm(form)
			return nil
		})
	})
	return result, err
}

func (r *FormRepository) List(ctx context.Context, tenantID, entityID string) ([]model.FormDefinition, error) {
	result := []model.FormDefinition{}
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var forms []model.Form
		query := db.Where("tenant_id = ? AND status = ?", tenantID, "active")
		if entityID != "" {
			query = query.Where("entity_id = ?", entityID)
		}
		if err := query.Order("name").Find(&forms).Error; err != nil {
			return err
		}
		for _, form := range forms {
			result = append(result, assembleForm(form))
		}
		return nil
	})
	return result, err
}

func (r *FormRepository) Get(ctx context.Context, tenantID, formID string) (model.FormDefinition, error) {
	var result model.FormDefinition
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var form model.Form
		if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, formID, "active").First(&form).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFormNotFound
			}
			return err
		}
		result = assembleForm(form)
		return nil
	})
	return result, err
}

func (r *FormRepository) Archive(ctx context.Context, tenantID, formID string, expectedVersion int) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		update := db.Model(&model.Form{}).Where("tenant_id = ? AND id = ? AND status = ? AND version = ?", tenantID, formID, "active", expectedVersion).Updates(map[string]any{"status": "archived", "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			var count int64
			if err := db.Model(&model.Form{}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, formID, "active").Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrFormNotFound
			}
			return ErrFormConflict
		}
		return nil
	})
}

func assembleForm(form model.Form) model.FormDefinition {
	schema := map[string]any{}
	components := []model.FormComponent{}
	_ = json.Unmarshal(form.JSONSchema, &schema)
	_ = json.Unmarshal(form.Layout, &components)
	return model.FormDefinition{ID: form.ID, EntityID: form.EntityID, Name: form.Name, Slug: form.Slug, Description: form.Description, JSONSchema: schema, Components: components, Version: form.Version, Status: form.Status, CreatedAt: form.CreatedAt, UpdatedAt: form.UpdatedAt}
}
