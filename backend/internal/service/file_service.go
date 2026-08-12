package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

var ErrInvalidFile = errors.New("invalid file")

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Provider() string
}

type FileRepository interface {
	Create(context.Context, model.FileAsset) (model.FileAsset, error)
	List(context.Context, string) ([]model.FileAsset, error)
	Get(context.Context, string, string) (model.FileAsset, error)
	Archive(context.Context, string, string) (model.FileAsset, error)
}

type FileService interface {
	Upload(context.Context, string, string, string, string, int64, io.Reader) (model.FileAsset, error)
	List(context.Context, string) ([]model.FileAsset, error)
	Open(context.Context, string, string) (model.FileAsset, io.ReadCloser, error)
	Delete(context.Context, string, string) error
}

type fileService struct {
	repository FileRepository
	store      ObjectStore
}

func NewFileService(repository FileRepository, store ObjectStore) FileService {
	return &fileService{repository: repository, store: store}
}

func (s *fileService) Upload(ctx context.Context, tenantID, actorID, name, contentType string, size int64, reader io.Reader) (model.FileAsset, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" || len(name) > 255 || size < 1 || size > 50<<20 {
		return model.FileAsset{}, fmt.Errorf("%w: name or size is invalid", ErrInvalidFile)
	}
	if !allowedFileType(name, contentType) {
		return model.FileAsset{}, fmt.Errorf("%w: supported files are images, PDF, and Excel", ErrInvalidFile)
	}
	id := newServiceUUID()
	if id == "" {
		return model.FileAsset{}, errors.New("generate file identifier")
	}
	key := tenantID + "/" + id + filepath.Ext(name)
	if err := s.store.Put(ctx, key, io.LimitReader(reader, (50<<20)+1), size, contentType); err != nil {
		return model.FileAsset{}, err
	}
	asset := model.FileAsset{ID: id, TenantID: tenantID, Name: name, ContentType: contentType, Size: size, StorageKey: key, Provider: s.store.Provider(), Status: "active", CreatedBy: actorID}
	asset, err := s.repository.Create(ctx, asset)
	if err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), key)
		return model.FileAsset{}, err
	}
	return asset, nil
}

func (s *fileService) List(ctx context.Context, tenantID string) ([]model.FileAsset, error) {
	return s.repository.List(ctx, tenantID)
}

func (s *fileService) Open(ctx context.Context, tenantID, id string) (model.FileAsset, io.ReadCloser, error) {
	asset, err := s.repository.Get(ctx, tenantID, id)
	if err != nil {
		return model.FileAsset{}, nil, err
	}
	reader, err := s.store.Open(ctx, asset.StorageKey)
	return asset, reader, err
}

func (s *fileService) Delete(ctx context.Context, tenantID, id string) error {
	asset, err := s.repository.Archive(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.store.Delete(ctx, asset.StorageKey)
}

func allowedFileType(name, contentType string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	allowedExtensions := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".pdf": true, ".xls": true, ".xlsx": true}
	if !allowedExtensions[ext] {
		return false
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return strings.HasPrefix(contentType, "image/") || contentType == "application/pdf" || contentType == "application/vnd.ms-excel" || contentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || contentType == "application/octet-stream"
}
