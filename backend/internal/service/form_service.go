package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

var ErrInvalidForm = errors.New("invalid form definition")

type FormRepository interface {
	Define(ctx context.Context, tenantID string, input model.DefineFormInput, jsonSchema map[string]any) (model.FormDefinition, error)
	List(ctx context.Context, tenantID, entityID string) ([]model.FormDefinition, error)
	Get(ctx context.Context, tenantID, formID string) (model.FormDefinition, error)
	Archive(ctx context.Context, tenantID, formID string, expectedVersion int) error
}

type FormService interface {
	Define(ctx context.Context, tenantID string, input model.DefineFormInput) (model.FormDefinition, error)
	List(ctx context.Context, tenantID, entityID string) ([]model.FormDefinition, error)
	Get(ctx context.Context, tenantID, formID string) (model.FormDefinition, error)
	Archive(ctx context.Context, tenantID, formID string, expectedVersion int) error
}

type formService struct {
	repository FormRepository
	models     DynamicModelService
}

func NewFormService(repository FormRepository, models DynamicModelService) FormService {
	return &formService{repository: repository, models: models}
}

func (s *formService) Define(ctx context.Context, tenantID string, input model.DefineFormInput) (model.FormDefinition, error) {
	input.Name, input.Slug, input.Description = strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Slug)), strings.TrimSpace(input.Description)
	if len(input.Name) < 2 || len(input.Name) > 120 || !entitySlugPattern.MatchString(input.Slug) || len(input.Description) > 500 {
		return model.FormDefinition{}, fmt.Errorf("%w: invalid name, slug, or description", ErrInvalidForm)
	}
	if input.ID != "" && input.ExpectedVersion < 1 {
		return model.FormDefinition{}, fmt.Errorf("%w: expected_version is required", ErrInvalidForm)
	}
	if input.ID != "" {
		existing, err := s.repository.Get(ctx, tenantID, input.ID)
		if err != nil {
			return model.FormDefinition{}, err
		}
		if existing.EntityID != input.EntityID {
			return model.FormDefinition{}, fmt.Errorf("%w: a saved form cannot change entity", ErrInvalidForm)
		}
	}
	schema, err := s.models.Get(ctx, tenantID, input.EntityID)
	if err != nil {
		return model.FormDefinition{}, err
	}
	jsonSchema, err := validateAndGenerateForm(schema, &input)
	if err != nil {
		return model.FormDefinition{}, err
	}
	return s.repository.Define(ctx, tenantID, input, jsonSchema)
}
func (s *formService) List(ctx context.Context, tenantID, entityID string) ([]model.FormDefinition, error) {
	return s.repository.List(ctx, tenantID, entityID)
}
func (s *formService) Get(ctx context.Context, tenantID, formID string) (model.FormDefinition, error) {
	return s.repository.Get(ctx, tenantID, formID)
}
func (s *formService) Archive(ctx context.Context, tenantID, formID string, expectedVersion int) error {
	if expectedVersion < 1 {
		return fmt.Errorf("%w: expected_version is required", ErrInvalidForm)
	}
	return s.repository.Archive(ctx, tenantID, formID, expectedVersion)
}

var supportedWidgets = map[string]struct{}{"input": {}, "textarea": {}, "date": {}, "datetime": {}, "select": {}, "multiselect": {}, "money": {}, "user": {}, "image": {}, "attachment": {}, "checkbox": {}}
var safeProp = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func validateAndGenerateForm(entity model.EntityDefinition, input *model.DefineFormInput) (map[string]any, error) {
	if len(input.Components) == 0 || len(input.Components) > 100 {
		return nil, fmt.Errorf("%w: form must contain 1-100 components", ErrInvalidForm)
	}
	fields := make(map[string]model.FieldDefinition, len(entity.Fields))
	for _, field := range entity.Fields {
		fields[field.Name] = field
	}
	properties := map[string]any{}
	required := []string{}
	seen := map[string]struct{}{}
	for index := range input.Components {
		component := &input.Components[index]
		component.FieldName, component.Widget, component.Label, component.Position = strings.TrimSpace(component.FieldName), strings.ToLower(strings.TrimSpace(component.Widget)), strings.TrimSpace(component.Label), index
		field, exists := fields[component.FieldName]
		if !exists {
			return nil, fmt.Errorf("%w: unknown field %q", ErrInvalidForm, component.FieldName)
		}
		if _, duplicate := seen[component.FieldName]; duplicate {
			return nil, fmt.Errorf("%w: duplicate field %q", ErrInvalidForm, component.FieldName)
		}
		seen[component.FieldName] = struct{}{}
		if _, ok := supportedWidgets[component.Widget]; !ok || !widgetSupports(component.Widget, field.Type) {
			return nil, fmt.Errorf("%w: widget %q is incompatible with field %q", ErrInvalidForm, component.Widget, field.Name)
		}
		if component.Label == "" {
			component.Label = field.Label
		}
		if len(component.Label) > 120 {
			return nil, fmt.Errorf("%w: component label is too long", ErrInvalidForm)
		}
		for key := range component.Props {
			if !safeProp.MatchString(key) {
				return nil, fmt.Errorf("%w: invalid component property %q", ErrInvalidForm, key)
			}
		}
		property := fieldJSONSchema(field, component.Label)
		properties[field.Name] = property
		if component.Required || field.Required {
			component.Required = true
			required = append(required, field.Name)
		}
	}
	result := map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "title": input.Name, "additionalProperties": false, "properties": properties}
	if len(required) > 0 {
		result["required"] = required
	}
	return result, nil
}

func widgetSupports(widget, fieldType string) bool {
	allowed := map[string][]string{
		"input": {"string", "email", "url"}, "textarea": {"string", "text"}, "date": {"date"}, "datetime": {"datetime"},
		"select": {"select"}, "multiselect": {"multiselect"}, "money": {"money", "number"}, "user": {"user"},
		"image": {"image"}, "attachment": {"attachment"}, "checkbox": {"boolean"},
	}
	for _, value := range allowed[widget] {
		if value == fieldType {
			return true
		}
	}
	return false
}

func fieldJSONSchema(field model.FieldDefinition, title string) map[string]any {
	property := map[string]any{"title": title}
	switch field.Type {
	case "number", "money":
		property["type"] = "number"
	case "boolean":
		property["type"] = "boolean"
	case "multiselect":
		property["type"] = "array"
		property["items"] = map[string]any{"type": "string", "enum": field.Options}
	default:
		property["type"] = "string"
	}
	formats := map[string]string{"date": "date", "datetime": "date-time", "email": "email", "url": "uri"}
	if format := formats[field.Type]; format != "" {
		property["format"] = format
	}
	if field.Type == "select" {
		property["enum"] = field.Options
	}
	if field.Default != nil {
		property["default"] = field.Default
	}
	return property
}
