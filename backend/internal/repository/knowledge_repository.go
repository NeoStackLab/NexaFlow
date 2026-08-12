package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
	"gorm.io/gorm"
)

var ErrKnowledgeNotFound = errors.New("knowledge document not found")

type KnowledgeRepository struct{ install InstallRepository }

func NewKnowledgeRepository(install InstallRepository) *KnowledgeRepository {
	return &KnowledgeRepository{install: install}
}
func (r *KnowledgeRepository) withDB(ctx context.Context, fn func(*gorm.DB) error) error {
	runtimeConfig, err := r.install.RuntimeConfig()
	if err != nil {
		return err
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
func (r *KnowledgeRepository) Save(ctx context.Context, tenantID, actorID, name, contentType string, size int64, chunks []model.KnowledgeChunkInput) (model.KnowledgeDocument, error) {
	var document model.KnowledgeDocument
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			document = model.KnowledgeDocument{ID: newUUID(), TenantID: tenantID, Name: name, ContentType: contentType, Size: size, ChunkCount: len(chunks), Status: "active", CreatedBy: actorID, CreatedAt: now}
			if err := tx.Create(&document).Error; err != nil {
				return err
			}
			for _, input := range chunks {
				chunk := model.KnowledgeChunk{ID: newUUID(), TenantID: tenantID, DocumentID: document.ID, Position: input.Position, Content: input.Content, Embedding: vectorLiteral(input.Embedding), CreatedAt: now}
				if err := tx.Create(&chunk).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
	return document, err
}
func (r *KnowledgeRepository) List(ctx context.Context, tenantID string) ([]model.KnowledgeDocument, error) {
	result := []model.KnowledgeDocument{}
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Where("tenant_id = ? AND status = ?", tenantID, "active").Order("created_at DESC").Find(&result).Error
	})
	return result, err
}
func (r *KnowledgeRepository) Delete(ctx context.Context, tenantID, id string) (int64, error) {
	var size int64
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var document model.KnowledgeDocument
			if err := tx.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, "active").First(&document).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrKnowledgeNotFound
				}
				return err
			}
			update := tx.Model(&model.KnowledgeDocument{}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, "active").Update("status", "archived")
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				return ErrKnowledgeNotFound
			}
			size = document.Size
			return tx.Where("tenant_id = ? AND document_id = ?", tenantID, id).Delete(&model.KnowledgeChunk{}).Error
		})
	})
	return size, err
}
func (r *KnowledgeRepository) Search(ctx context.Context, tenantID string, embedding []float32, limit int) ([]model.KnowledgeSource, error) {
	result := []model.KnowledgeSource{}
	vector := vectorLiteral(embedding)
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Raw(`SELECT c.document_id, d.name AS document_name, c.id AS chunk_id, c.content, 1 - (c.embedding <=> ?::vector) AS score FROM knowledge_chunks c JOIN knowledge_documents d ON d.id = c.document_id AND d.tenant_id = c.tenant_id WHERE c.tenant_id = ? AND d.status = 'active' ORDER BY c.embedding <=> ?::vector LIMIT ?`, vector, tenantID, vector, limit).Scan(&result).Error
	})
	return result, err
}
func vectorLiteral(values []float32) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("%g", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
