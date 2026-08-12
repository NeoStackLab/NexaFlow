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
	ErrEntityNotFound = errors.New("entity not found")
	ErrEntityConflict = errors.New("entity schema version conflict")
	ErrEntitySlugUsed = errors.New("entity slug already exists")
)

type DynamicModelRepository interface {
	Define(ctx context.Context, tenantID string, input model.DefineEntityInput) (model.EntityDefinition, error)
	List(ctx context.Context, tenantID string) ([]model.EntityDefinition, error)
	Get(ctx context.Context, tenantID, entityID string) (model.EntityDefinition, error)
	Archive(ctx context.Context, tenantID, entityID string, expectedVersion int) error
}

type dynamicModelRepository struct{ install InstallRepository }

func NewDynamicModelRepository(install InstallRepository) DynamicModelRepository {
	return &dynamicModelRepository{install: install}
}

func (r *dynamicModelRepository) withDB(ctx context.Context, fn func(*gorm.DB) error) error {
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

func (r *dynamicModelRepository) Define(ctx context.Context, tenantID string, input model.DefineEntityInput) (model.EntityDefinition, error) {
	var result model.EntityDefinition
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			entity := model.Entity{ID: input.ID, TenantID: tenantID, Name: input.Name, Slug: input.Slug, Description: input.Description, Status: "active", UpdatedAt: now}
			if input.ID == "" {
				var count int64
				if err := tx.Model(&model.Entity{}).Where("tenant_id = ? AND slug = ?", tenantID, input.Slug).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return ErrEntitySlugUsed
				}
				entity.ID, entity.Version, entity.CreatedAt = newUUID(), 1, now
				if err := tx.Create(&entity).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, input.ID, "active").First(&entity).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return ErrEntityNotFound
					}
					return err
				}
				if input.ExpectedVersion < 1 || entity.Version != input.ExpectedVersion {
					return ErrEntityConflict
				}
				var count int64
				if err := tx.Model(&model.Entity{}).Where("tenant_id = ? AND slug = ? AND id <> ?", tenantID, input.Slug, input.ID).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return ErrEntitySlugUsed
				}
				entity.Name, entity.Slug, entity.Description, entity.Version, entity.UpdatedAt = input.Name, input.Slug, input.Description, entity.Version+1, now
				if err := tx.Model(&entity).Select("name", "slug", "description", "version", "updated_at").Updates(&entity).Error; err != nil {
					return err
				}
				if err := tx.Where("tenant_id = ? AND entity_id = ?", tenantID, entity.ID).Delete(&model.EntityField{}).Error; err != nil {
					return err
				}
			}
			fields := make([]model.EntityField, 0, len(input.Fields))
			for position, definition := range input.Fields {
				defaultValue, err := json.Marshal(definition.Default)
				if err != nil {
					return fmt.Errorf("encode default for %s: %w", definition.Name, err)
				}
				if definition.Default == nil {
					defaultValue = nil
				}
				options, err := json.Marshal(definition.Options)
				if err != nil {
					return fmt.Errorf("encode options for %s: %w", definition.Name, err)
				}
				field := model.EntityField{ID: newUUID(), TenantID: tenantID, EntityID: entity.ID, Name: definition.Name, Label: definition.Label, Type: definition.Type, Required: definition.Required, DefaultValue: defaultValue, Options: options, Position: position, CreatedAt: now, UpdatedAt: now}
				if err := tx.Create(&field).Error; err != nil {
					return err
				}
				fields = append(fields, field)
			}
			result = assembleEntity(entity, fields)
			return nil
		})
	})
	return result, err
}

func (r *dynamicModelRepository) List(ctx context.Context, tenantID string) ([]model.EntityDefinition, error) {
	result := []model.EntityDefinition{}
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var entities []model.Entity
		if err := db.Where("tenant_id = ? AND status = ?", tenantID, "active").Order("name").Find(&entities).Error; err != nil {
			return err
		}
		for _, entity := range entities {
			fields, err := loadEntityFields(db, tenantID, entity.ID)
			if err != nil {
				return err
			}
			result = append(result, assembleEntity(entity, fields))
		}
		return nil
	})
	return result, err
}

func (r *dynamicModelRepository) Get(ctx context.Context, tenantID, entityID string) (model.EntityDefinition, error) {
	var result model.EntityDefinition
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var entity model.Entity
		if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, entityID, "active").First(&entity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEntityNotFound
			}
			return err
		}
		fields, err := loadEntityFields(db, tenantID, entity.ID)
		if err != nil {
			return err
		}
		result = assembleEntity(entity, fields)
		return nil
	})
	return result, err
}

func (r *dynamicModelRepository) Archive(ctx context.Context, tenantID, entityID string, expectedVersion int) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		result := db.Model(&model.Entity{}).Where("tenant_id = ? AND id = ? AND status = ? AND version = ?", tenantID, entityID, "active", expectedVersion).Updates(map[string]any{"status": "archived", "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var count int64
			if err := db.Model(&model.Entity{}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, entityID, "active").Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrEntityNotFound
			}
			return ErrEntityConflict
		}
		return nil
	})
}

func loadEntityFields(db *gorm.DB, tenantID, entityID string) ([]model.EntityField, error) {
	var fields []model.EntityField
	err := db.Where("tenant_id = ? AND entity_id = ?", tenantID, entityID).Order("position").Find(&fields).Error
	return fields, err
}
func assembleEntity(entity model.Entity, fields []model.EntityField) model.EntityDefinition {
	definitions := make([]model.FieldDefinition, 0, len(fields))
	for _, field := range fields {
		var defaultValue any
		if len(field.DefaultValue) > 0 {
			_ = json.Unmarshal(field.DefaultValue, &defaultValue)
		}
		options := []string{}
		if len(field.Options) > 0 {
			_ = json.Unmarshal(field.Options, &options)
		}
		definitions = append(definitions, model.FieldDefinition{ID: field.ID, Name: field.Name, Label: field.Label, Type: field.Type, Required: field.Required, Default: defaultValue, Options: options, Position: field.Position})
	}
	return model.EntityDefinition{ID: entity.ID, TenantID: entity.TenantID, Name: entity.Name, Slug: entity.Slug, Description: entity.Description, Version: entity.Version, Status: entity.Status, Fields: definitions, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
}
