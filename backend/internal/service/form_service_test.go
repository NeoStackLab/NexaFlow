package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type formRepositoryStub struct {
	input    model.DefineFormInput
	schema   map[string]any
	calls    int
	existing model.FormDefinition
}

func (s *formRepositoryStub) Define(_ context.Context, _ string, input model.DefineFormInput, schema map[string]any) (model.FormDefinition, error) {
	s.calls++
	s.input, s.schema = input, schema
	return model.FormDefinition{JSONSchema: schema, Components: input.Components}, nil
}
func (*formRepositoryStub) List(context.Context, string, string) ([]model.FormDefinition, error) {
	return nil, nil
}
func (s *formRepositoryStub) Get(context.Context, string, string) (model.FormDefinition, error) {
	return s.existing, nil
}

func TestFormServicePreventsChangingEntityOnUpdate(t *testing.T) {
	repository := &formRepositoryStub{existing: model.FormDefinition{ID: "form-1", EntityID: "entity-1"}}
	service := NewFormService(repository, &modelServiceStub{schema: recordSchema()})
	_, err := service.Define(context.Background(), "tenant-a", model.DefineFormInput{ID: "form-1", EntityID: "entity-2", ExpectedVersion: 1, Name: "Customer form", Slug: "customer-form", Components: []model.FormComponent{{FieldName: "name", Widget: "input"}}})
	if !errors.Is(err, ErrInvalidForm) || repository.calls != 0 {
		t.Fatalf("Define() error=%v calls=%d", err, repository.calls)
	}
}
func (*formRepositoryStub) Archive(context.Context, string, string, int) error { return nil }

func TestFormServiceGeneratesJSONSchemaAndPositions(t *testing.T) {
	repository := &formRepositoryStub{}
	service := NewFormService(repository, &modelServiceStub{schema: recordSchema()})
	result, err := service.Define(context.Background(), "tenant-a", model.DefineFormInput{EntityID: "entity-1", Name: "Customer form", Slug: "customer-form", Components: []model.FormComponent{{FieldName: "tier", Widget: "select", Label: "Level"}, {FieldName: "name", Widget: "input", Required: true}}})
	if err != nil {
		t.Fatalf("Define() error = %v", err)
	}
	if result.Components[1].Position != 1 || result.JSONSchema["additionalProperties"] != false {
		t.Fatalf("result = %#v", result)
	}
	properties := result.JSONSchema["properties"].(map[string]any)
	if properties["tier"].(map[string]any)["title"] != "Level" {
		t.Fatalf("schema = %#v", result.JSONSchema)
	}
}

func TestFormServiceRejectsUnknownDuplicateAndIncompatibleComponents(t *testing.T) {
	tests := [][]model.FormComponent{{{FieldName: "missing", Widget: "input"}}, {{FieldName: "name", Widget: "input"}, {FieldName: "name", Widget: "textarea"}}, {{FieldName: "tier", Widget: "date"}}}
	for _, components := range tests {
		repository := &formRepositoryStub{}
		service := NewFormService(repository, &modelServiceStub{schema: recordSchema()})
		_, err := service.Define(context.Background(), "tenant-a", model.DefineFormInput{EntityID: "entity-1", Name: "Customer form", Slug: "customer-form", Components: components})
		if !errors.Is(err, ErrInvalidForm) || repository.calls != 0 {
			t.Fatalf("components=%#v error=%v calls=%d", components, err, repository.calls)
		}
	}
}
