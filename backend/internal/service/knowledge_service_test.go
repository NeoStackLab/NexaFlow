package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type knowledgeRepositoryStub struct {
	saved      bool
	deleteSize int64
}

func (s *knowledgeRepositoryStub) Save(context.Context, string, string, string, string, int64, []model.KnowledgeChunkInput) (model.KnowledgeDocument, error) {
	s.saved = true
	return model.KnowledgeDocument{ID: "document-1"}, nil
}
func (*knowledgeRepositoryStub) List(context.Context, string) ([]model.KnowledgeDocument, error) {
	return nil, nil
}
func (s *knowledgeRepositoryStub) Delete(context.Context, string, string) (int64, error) {
	return s.deleteSize, nil
}
func (*knowledgeRepositoryStub) Search(context.Context, string, []float32, int) ([]model.KnowledgeSource, error) {
	return nil, nil
}

type knowledgeProviderStub struct{ embeds int }

func (s *knowledgeProviderStub) Embed(_ context.Context, texts []string) ([][]float32, error) {
	s.embeds++
	result := make([][]float32, len(texts))
	for index := range result {
		result[index] = []float32{1}
	}
	return result, nil
}
func (*knowledgeProviderStub) Complete(context.Context, string, string) (string, int, int, error) {
	return "", 0, 0, nil
}

type usageMeterStub struct {
	quantities []int64
	reject     bool
}

func (s *usageMeterStub) Consume(_ context.Context, _, _ string, quantity int64) error {
	s.quantities = append(s.quantities, quantity)
	if s.reject && quantity > 0 {
		return ErrQuotaExceeded
	}
	return nil
}

func TestKnowledgeQuotaIsReservedBeforeEmbedding(t *testing.T) {
	repository, provider, meter := &knowledgeRepositoryStub{}, &knowledgeProviderStub{}, &usageMeterStub{reject: true}
	service := NewKnowledgeService(repository, provider, meter)
	_, err := service.Ingest(context.Background(), "tenant-a", "user-1", "policy.txt", "text/plain", 32, strings.NewReader("This is enough valid policy text for testing."))
	if !errors.Is(err, ErrQuotaExceeded) || provider.embeds != 0 || repository.saved {
		t.Fatalf("error=%v embeds=%d saved=%v", err, provider.embeds, repository.saved)
	}
}

func TestKnowledgeReservationIsReleasedWhenParsingFails(t *testing.T) {
	meter := &usageMeterStub{}
	service := NewKnowledgeService(&knowledgeRepositoryStub{}, &knowledgeProviderStub{}, meter)
	_, err := service.Ingest(context.Background(), "tenant-a", "user-1", "unsupported.exe", "application/octet-stream", 8, io.LimitReader(strings.NewReader("bad data"), 8))
	if !errors.Is(err, ErrInvalidKnowledgeDocument) || len(meter.quantities) != 2 || meter.quantities[0] != 8 || meter.quantities[1] != -8 {
		t.Fatalf("error=%v quantities=%v", err, meter.quantities)
	}
}

func TestKnowledgeDeleteReleasesStorageUsage(t *testing.T) {
	meter := &usageMeterStub{}
	service := NewKnowledgeService(&knowledgeRepositoryStub{deleteSize: 2048}, &knowledgeProviderStub{}, meter)
	if err := service.Delete(context.Background(), "tenant-a", "document-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(meter.quantities) != 1 || meter.quantities[0] != -2048 {
		t.Fatalf("quantities=%v", meter.quantities)
	}
}
