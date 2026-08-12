package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type dynamicModelRepositoryStub struct {
	defineInput    model.DefineEntityInput
	defineCalls    int
	archiveVersion int
}

func (s *dynamicModelRepositoryStub) Define(_ context.Context, _ string, input model.DefineEntityInput) (model.EntityDefinition, error) {
	s.defineCalls++
	s.defineInput = input
	return model.EntityDefinition{ID: "entity-1", Name: input.Name, Slug: input.Slug, Fields: input.Fields}, nil
}
func (s *dynamicModelRepositoryStub) List(context.Context, string) ([]model.EntityDefinition, error) {
	return nil, nil
}
func (s *dynamicModelRepositoryStub) Get(context.Context, string, string) (model.EntityDefinition, error) {
	return model.EntityDefinition{}, nil
}
func (s *dynamicModelRepositoryStub) Archive(_ context.Context, _, _ string, version int) error {
	s.archiveVersion = version
	return nil
}

func validEntityInput() model.DefineEntityInput {
	return model.DefineEntityInput{Name: "Customer", Slug: "customers", Description: "Customer master data", Fields: []model.FieldDefinition{{Name: "display_name", Label: "Display name", Type: "string", Required: true}, {Name: "tier", Label: "Tier", Type: "select", Options: []string{"standard", "vip"}, Default: "standard"}}}
}

func TestDynamicModelServiceNormalizesValidSchema(t *testing.T) {
	repository := &dynamicModelRepositoryStub{}
	service := NewDynamicModelService(repository)
	input := validEntityInput()
	input.Name = " Customer "
	input.Fields[0].Name = " DISPLAY_NAME "
	result, err := service.Define(context.Background(), "tenant-a", input)
	if err != nil {
		t.Fatalf("Define() error = %v", err)
	}
	if result.Name != "Customer" || repository.defineInput.Fields[0].Name != "display_name" {
		t.Fatalf("normalized result/input = %#v / %#v", result, repository.defineInput)
	}
	if repository.defineInput.Fields[1].Position != 1 {
		t.Fatalf("field position = %d, want 1", repository.defineInput.Fields[1].Position)
	}
}

func TestDynamicModelServiceRejectsReservedAndDuplicateFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*model.DefineEntityInput)
	}{
		{"reserved", func(input *model.DefineEntityInput) { input.Fields[0].Name = "tenant_id" }},
		{"duplicate", func(input *model.DefineEntityInput) { input.Fields[1].Name = "display_name" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &dynamicModelRepositoryStub{}
			service := NewDynamicModelService(repository)
			input := validEntityInput()
			test.mutate(&input)
			_, err := service.Define(context.Background(), "tenant-a", input)
			if !errors.Is(err, ErrInvalidEntitySchema) {
				t.Fatalf("Define() error = %v", err)
			}
			if repository.defineCalls != 0 {
				t.Fatal("invalid schema crossed repository seam")
			}
		})
	}
}

func TestDynamicModelServiceValidatesOptionsAndDefaults(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*model.DefineEntityInput)
	}{
		{"select without options", func(input *model.DefineEntityInput) { input.Fields[1].Options = nil }},
		{"options on string", func(input *model.DefineEntityInput) { input.Fields[0].Options = []string{"x"} }},
		{"unknown select default", func(input *model.DefineEntityInput) { input.Fields[1].Default = "enterprise" }},
		{"wrong number default", func(input *model.DefineEntityInput) { input.Fields[0].Type = "number"; input.Fields[0].Default = "42" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &dynamicModelRepositoryStub{}
			service := NewDynamicModelService(repository)
			input := validEntityInput()
			test.mutate(&input)
			_, err := service.Define(context.Background(), "tenant-a", input)
			if !errors.Is(err, ErrInvalidEntitySchema) {
				t.Fatalf("Define() error = %v", err)
			}
		})
	}
}

func TestDynamicModelServiceRequiresArchiveVersion(t *testing.T) {
	repository := &dynamicModelRepositoryStub{}
	service := NewDynamicModelService(repository)
	if err := service.Archive(context.Background(), "tenant-a", "entity-1", 0); !errors.Is(err, ErrInvalidEntitySchema) {
		t.Fatalf("Archive() error = %v", err)
	}
	if repository.archiveVersion != 0 {
		t.Fatal("invalid archive crossed repository seam")
	}
}
