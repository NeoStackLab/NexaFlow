package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type fileRepositoryStub struct{ created model.FileAsset }

func (s *fileRepositoryStub) Create(_ context.Context, asset model.FileAsset) (model.FileAsset, error) {
	s.created = asset
	return asset, nil
}
func (*fileRepositoryStub) List(context.Context, string) ([]model.FileAsset, error) { return nil, nil }
func (*fileRepositoryStub) Get(context.Context, string, string) (model.FileAsset, error) {
	return model.FileAsset{}, nil
}
func (*fileRepositoryStub) Archive(context.Context, string, string) (model.FileAsset, error) {
	return model.FileAsset{}, nil
}

func TestFileServiceStoresOnlyAllowedTenantScopedFiles(t *testing.T) {
	root := t.TempDir()
	repository := &fileRepositoryStub{}
	service := NewFileService(repository, &localObjectStore{root: root})
	content := "spreadsheet-content"
	asset, err := service.Upload(context.Background(), "tenant-a", "user-1", "report.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", int64(len(content)), strings.NewReader(content))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if !strings.HasPrefix(asset.StorageKey, "tenant-a/") || repository.created.ID == "" {
		t.Fatalf("asset=%#v", asset)
	}
	stored, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(asset.StorageKey)))
	if err != nil || string(stored) != content {
		t.Fatalf("stored=%q error=%v", stored, err)
	}
}

func TestFileServiceRejectsExecutableBeforeStorage(t *testing.T) {
	service := NewFileService(&fileRepositoryStub{}, &localObjectStore{root: t.TempDir()})
	if _, err := service.Upload(context.Background(), "tenant-a", "user-1", "payload.exe", "application/octet-stream", 3, strings.NewReader("bad")); err == nil {
		t.Fatal("executable upload was accepted")
	}
}

func TestLocalObjectStoreRejectsPathTraversal(t *testing.T) {
	store := &localObjectStore{root: t.TempDir()}
	if err := store.Put(context.Background(), "../escape.pdf", strings.NewReader("x"), 1, "application/pdf"); err == nil {
		t.Fatal("path traversal key was accepted")
	}
	if _, err := store.Open(context.Background(), "../escape.pdf"); err == nil {
		t.Fatal("path traversal open was accepted")
	}
}
