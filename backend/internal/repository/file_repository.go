package repository

import (
	"context"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
)

var ErrFileNotFound = errors.New("file not found")

type FileRepository struct{ knowledge *KnowledgeRepository }

func NewFileRepository(install InstallRepository) *FileRepository {
	return &FileRepository{knowledge: NewKnowledgeRepository(install)}
}

func (r *FileRepository) Create(ctx context.Context, asset model.FileAsset) (model.FileAsset, error) {
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		asset.CreatedAt = time.Now().UTC()
		return db.Create(&asset).Error
	})
	return asset, err
}

func (r *FileRepository) List(ctx context.Context, tenantID string) ([]model.FileAsset, error) {
	result := []model.FileAsset{}
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		return db.Where("tenant_id = ? AND status = ?", tenantID, "active").Order("created_at DESC").Find(&result).Error
	})
	return result, err
}

func (r *FileRepository) Get(ctx context.Context, tenantID, id string) (model.FileAsset, error) {
	var asset model.FileAsset
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, "active").First(&asset).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return err
		}
		return nil
	})
	return asset, err
}

func (r *FileRepository) Archive(ctx context.Context, tenantID, id string) (model.FileAsset, error) {
	var asset model.FileAsset
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, "active").First(&asset).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrFileNotFound
				}
				return err
			}
			return tx.Model(&asset).Update("status", "archived").Error
		})
	})
	return asset, err
}
