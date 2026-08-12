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
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrWorkflowConflict = errors.New("workflow version conflict")
	ErrWorkflowSlugUsed = errors.New("workflow slug already exists")
	ErrInstanceNotFound = errors.New("workflow instance not found")
	ErrInstanceConflict = errors.New("workflow instance version conflict")
)

type WorkflowRepository struct{ install InstallRepository }

func NewWorkflowRepository(install InstallRepository) *WorkflowRepository {
	return &WorkflowRepository{install: install}
}
func (r *WorkflowRepository) withDB(ctx context.Context, fn func(*gorm.DB) error) error {
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

func (r *WorkflowRepository) Define(ctx context.Context, tenantID string, input model.DefineWorkflowInput) (model.WorkflowDefinition, error) {
	var result model.WorkflowDefinition
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			nodes, err := json.Marshal(input.Nodes)
			if err != nil {
				return err
			}
			edges, err := json.Marshal(input.Edges)
			if err != nil {
				return err
			}
			workflow := model.Workflow{ID: input.ID, TenantID: tenantID, EntityID: input.EntityID, Name: input.Name, Slug: input.Slug, Description: input.Description, Nodes: nodes, Edges: edges, Status: "active", UpdatedAt: now}
			if input.ID == "" {
				var count int64
				if err := tx.Model(&model.Workflow{}).Where("tenant_id = ? AND slug = ?", tenantID, input.Slug).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return ErrWorkflowSlugUsed
				}
				workflow.ID, workflow.Version, workflow.CreatedAt = newUUID(), 1, now
				if err := tx.Create(&workflow).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, input.ID, "active").First(&workflow).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return ErrWorkflowNotFound
					}
					return err
				}
				if workflow.Version != input.ExpectedVersion {
					return ErrWorkflowConflict
				}
				if workflow.EntityID != input.EntityID {
					return ErrWorkflowConflict
				}
				var count int64
				if err := tx.Model(&model.Workflow{}).Where("tenant_id = ? AND slug = ? AND id <> ?", tenantID, input.Slug, input.ID).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return ErrWorkflowSlugUsed
				}
				workflow.Name, workflow.Slug, workflow.Description, workflow.Nodes, workflow.Edges, workflow.Version, workflow.UpdatedAt = input.Name, input.Slug, input.Description, nodes, edges, workflow.Version+1, now
				if err := tx.Model(&workflow).Select("name", "slug", "description", "nodes", "edges", "version", "updated_at").Updates(&workflow).Error; err != nil {
					return err
				}
			}
			result = assembleWorkflow(workflow)
			return nil
		})
	})
	return result, err
}
func (r *WorkflowRepository) List(ctx context.Context, tenantID string) ([]model.WorkflowDefinition, error) {
	result := []model.WorkflowDefinition{}
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var rows []model.Workflow
		if err := db.Where("tenant_id = ? AND status = ?", tenantID, "active").Order("name").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			result = append(result, assembleWorkflow(row))
		}
		return nil
	})
	return result, err
}
func (r *WorkflowRepository) Get(ctx context.Context, tenantID, id string) (model.WorkflowDefinition, error) {
	var result model.WorkflowDefinition
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var row model.Workflow
		if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, "active").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWorkflowNotFound
			}
			return err
		}
		result = assembleWorkflow(row)
		return nil
	})
	return result, err
}
func (r *WorkflowRepository) Archive(ctx context.Context, tenantID, id string, version int) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		update := db.Model(&model.Workflow{}).Where("tenant_id = ? AND id = ? AND status = ? AND version = ?", tenantID, id, "active", version).Updates(map[string]any{"status": "archived", "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			var count int64
			if err := db.Model(&model.Workflow{}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, "active").Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrWorkflowNotFound
			}
			return ErrWorkflowConflict
		}
		return nil
	})
}

func (r *WorkflowRepository) Start(ctx context.Context, tenantID string, definition model.WorkflowDefinition, recordID, actorID string, transition model.WorkflowTransition) (model.WorkflowInstance, error) {
	var instance model.WorkflowInstance
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			instance = model.WorkflowInstance{ID: newUUID(), TenantID: tenantID, WorkflowID: definition.ID, EntityID: definition.EntityID, RecordID: recordID, CurrentNodeID: transition.CurrentNodeID, Status: transition.Status, Version: 1, SubmittedBy: actorID, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
			return createNotifications(tx, tenantID, instance.ID, transition.Notifications, now)
		})
	})
	return instance, err
}
func (r *WorkflowRepository) ListInstances(ctx context.Context, tenantID, workflowID string) ([]model.WorkflowInstance, error) {
	result := []model.WorkflowInstance{}
	err := r.withDB(ctx, func(db *gorm.DB) error {
		query := db.Where("tenant_id = ?", tenantID)
		if workflowID != "" {
			query = query.Where("workflow_id = ?", workflowID)
		}
		return query.Order("created_at DESC").Limit(200).Find(&result).Error
	})
	return result, err
}
func (r *WorkflowRepository) GetInstance(ctx context.Context, tenantID, id string) (model.WorkflowInstance, error) {
	var instance model.WorkflowInstance
	err := r.withDB(ctx, func(db *gorm.DB) error {
		if err := db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&instance).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstanceNotFound
			}
			return err
		}
		return nil
	})
	return instance, err
}

func (r *WorkflowRepository) ListNotifications(ctx context.Context, tenantID, userID string) ([]model.Notification, error) {
	result := []model.Notification{}
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Where("tenant_id = ? AND user_id = ? AND channel = ?", tenantID, userID, "inapp").Order("created_at DESC").Limit(100).Find(&result).Error
	})
	return result, err
}

func (r *WorkflowRepository) ReadNotification(ctx context.Context, tenantID, userID, notificationID string) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		now := time.Now().UTC()
		result := db.Model(&model.Notification{}).Where("tenant_id = ? AND user_id = ? AND id = ? AND channel = ?", tenantID, userID, notificationID, "inapp").Updates(map[string]any{"status": "read", "read_at": &now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrInstanceNotFound
		}
		return nil
	})
}
func (r *WorkflowRepository) Advance(ctx context.Context, tenantID string, instance model.WorkflowInstance, actorID string, input model.WorkflowActionInput, transition model.WorkflowTransition) (model.WorkflowInstance, error) {
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			update := tx.Model(&model.WorkflowInstance{}).Where("tenant_id = ? AND id = ? AND status = ? AND version = ?", tenantID, instance.ID, "pending", input.ExpectedVersion).Updates(map[string]any{"current_node_id": transition.CurrentNodeID, "status": transition.Status, "version": gorm.Expr("version + 1"), "updated_at": now})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				return ErrInstanceConflict
			}
			action := model.WorkflowAction{ID: newUUID(), TenantID: tenantID, InstanceID: instance.ID, NodeID: instance.CurrentNodeID, ActorID: actorID, Action: input.Action, Comment: input.Comment, CreatedAt: now}
			if err := tx.Create(&action).Error; err != nil {
				return err
			}
			if err := createNotifications(tx, tenantID, instance.ID, transition.Notifications, now); err != nil {
				return err
			}
			instance.CurrentNodeID, instance.Status, instance.Version, instance.UpdatedAt = transition.CurrentNodeID, transition.Status, instance.Version+1, now
			return nil
		})
	})
	return instance, err
}

func createNotifications(tx *gorm.DB, tenantID, instanceID string, notifications []model.Notification, now time.Time) error {
	for _, notification := range notifications {
		notification.ID, notification.TenantID, notification.InstanceID, notification.CreatedAt = newUUID(), tenantID, instanceID, now
		if notification.UserID == "" {
			notification.UserID = "00000000-0000-0000-0000-000000000000"
		}
		if err := tx.Create(&notification).Error; err != nil {
			return err
		}
	}
	return nil
}
func assembleWorkflow(row model.Workflow) model.WorkflowDefinition {
	nodes := []model.WorkflowNode{}
	edges := []model.WorkflowEdge{}
	_ = json.Unmarshal(row.Nodes, &nodes)
	_ = json.Unmarshal(row.Edges, &edges)
	return model.WorkflowDefinition{ID: row.ID, EntityID: row.EntityID, Name: row.Name, Slug: row.Slug, Description: row.Description, Nodes: nodes, Edges: edges, Version: row.Version, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
