package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type recordRepositoryStub struct {
	values map[string]any
	calls  int
}

func (s *recordRepositoryStub) Create(_ context.Context, _, _, _ string, values map[string]any) (model.RecordView, error) {
	s.calls++
	s.values = values
	return model.RecordView{Values: values}, nil
}
func (*recordRepositoryStub) List(context.Context, string, string, int, int) ([]model.RecordView, int64, error) {
	return nil, 0, nil
}
func (*recordRepositoryStub) Get(context.Context, string, string, string) (model.RecordView, error) {
	return model.RecordView{}, nil
}
func (s *recordRepositoryStub) Update(_ context.Context, _, _, _, _ string, _ int, values map[string]any) (model.RecordView, error) {
	s.calls++
	s.values = values
	return model.RecordView{Values: values}, nil
}
func (*recordRepositoryStub) Delete(context.Context, string, string, string, int) error { return nil }

type modelServiceStub struct{ schema model.EntityDefinition }

func (*modelServiceStub) Define(context.Context, string, model.DefineEntityInput) (model.EntityDefinition, error) {
	return model.EntityDefinition{}, nil
}
func (*modelServiceStub) List(context.Context, string) ([]model.EntityDefinition, error) {
	return nil, nil
}
func (s *modelServiceStub) Get(context.Context, string, string) (model.EntityDefinition, error) {
	return s.schema, nil
}
func (*modelServiceStub) Archive(context.Context, string, string, int) error { return nil }

func recordSchema() model.EntityDefinition {
	return model.EntityDefinition{ID: "entity-1", Fields: []model.FieldDefinition{{Name: "name", Label: "Name", Type: "string", Required: true}, {Name: "tier", Label: "Tier", Type: "select", Options: []string{"standard", "vip"}, Default: "standard"}, {Name: "active", Label: "Active", Type: "boolean"}}}
}

func TestDynamicRecordServiceValidatesAndAppliesDefaults(t *testing.T) {
	repository := &recordRepositoryStub{}
	service := NewDynamicRecordService(repository, &modelServiceStub{schema: recordSchema()})
	result, err := service.Create(context.Background(), "tenant-a", "entity-1", "user-1", model.WriteRecordInput{Values: map[string]any{"name": "Acme", "active": true}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Values["tier"] != "standard" {
		t.Fatalf("default tier = %#v", result.Values["tier"])
	}
}

func TestDynamicRecordServiceRejectsInvalidValuesBeforeRepository(t *testing.T) {
	tests := []map[string]any{{"unknown": "x", "name": "Acme"}, {"tier": "vip"}, {"name": "Acme", "tier": "invalid"}, {"name": "Acme", "active": "yes"}}
	for _, values := range tests {
		repository := &recordRepositoryStub{}
		service := NewDynamicRecordService(repository, &modelServiceStub{schema: recordSchema()})
		_, err := service.Create(context.Background(), "tenant-a", "entity-1", "user-1", model.WriteRecordInput{Values: values})
		if !errors.Is(err, ErrInvalidRecord) || repository.calls != 0 {
			t.Fatalf("values=%#v error=%v calls=%d", values, err, repository.calls)
		}
	}
}
