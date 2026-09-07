package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
)

var ErrInvalidEntitySchema = errors.New("invalid entity schema")

var (
	entitySlugPattern   = regexp.MustCompile(`^[a-z][a-z0-9-]{1,78}[a-z0-9]$`)
	fieldNamePattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	reservedFields      = map[string]struct{}{"id": {}, "tenant_id": {}, "created_at": {}, "updated_at": {}, "deleted_at": {}}
	supportedFieldTypes = map[string]struct{}{"string": {}, "text": {}, "number": {}, "boolean": {}, "date": {}, "datetime": {}, "select": {}, "multiselect": {}, "money": {}, "email": {}, "url": {}, "user": {}, "image": {}, "attachment": {}}
)

type DynamicModelService interface {
	Define(ctx context.Context, tenantID string, input model.DefineEntityInput) (model.EntityDefinition, error)
	List(ctx context.Context, tenantID string) ([]model.EntityDefinition, error)
	Get(ctx context.Context, tenantID, entityID string) (model.EntityDefinition, error)
	Archive(ctx context.Context, tenantID, entityID string, expectedVersion int) error
}

type dynamicModelService struct {
	repository repository.DynamicModelRepository
}

func NewDynamicModelService(repository repository.DynamicModelRepository) DynamicModelService {
	return &dynamicModelService{repository: repository}
}

func (s *dynamicModelService) Define(ctx context.Context, tenantID string, input model.DefineEntityInput) (model.EntityDefinition, error) {
	input.Name, input.Slug, input.Description = strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Slug)), strings.TrimSpace(input.Description)
	if err := validateEntitySchema(input); err != nil {
		return model.EntityDefinition{}, err
	}
	for index := range input.Fields {
		input.Fields[index].Name = strings.ToLower(strings.TrimSpace(input.Fields[index].Name))
		input.Fields[index].Label = strings.TrimSpace(input.Fields[index].Label)
		input.Fields[index].Type = strings.ToLower(strings.TrimSpace(input.Fields[index].Type))
		input.Fields[index].Position = index
	}
	return s.repository.Define(ctx, tenantID, input)
}
func (s *dynamicModelService) List(ctx context.Context, tenantID string) ([]model.EntityDefinition, error) {
	return s.repository.List(ctx, tenantID)
}
func (s *dynamicModelService) Get(ctx context.Context, tenantID, entityID string) (model.EntityDefinition, error) {
	return s.repository.Get(ctx, tenantID, entityID)
}
func (s *dynamicModelService) Archive(ctx context.Context, tenantID, entityID string, expectedVersion int) error {
	if expectedVersion < 1 {
		return fmt.Errorf("%w: expected_version is required", ErrInvalidEntitySchema)
	}
	return s.repository.Archive(ctx, tenantID, entityID, expectedVersion)
}

func validateEntitySchema(input model.DefineEntityInput) error {
	if len(input.Name) < 2 || len(input.Name) > 120 {
		return fmt.Errorf("%w: entity name must be 2-120 characters", ErrInvalidEntitySchema)
	}
	if !entitySlugPattern.MatchString(input.Slug) {
		return fmt.Errorf("%w: slug must be 3-80 lowercase letters, numbers, or hyphens", ErrInvalidEntitySchema)
	}
	if len(input.Description) > 500 {
		return fmt.Errorf("%w: description exceeds 500 characters", ErrInvalidEntitySchema)
	}
	if len(input.Fields) == 0 || len(input.Fields) > 100 {
		return fmt.Errorf("%w: entity must contain 1-100 fields", ErrInvalidEntitySchema)
	}
	seen := make(map[string]struct{}, len(input.Fields))
	for _, raw := range input.Fields {
		field := raw
		field.Name, field.Label, field.Type = strings.ToLower(strings.TrimSpace(field.Name)), strings.TrimSpace(field.Label), strings.ToLower(strings.TrimSpace(field.Type))
		if !fieldNamePattern.MatchString(field.Name) {
			return fmt.Errorf("%w: invalid field name %q", ErrInvalidEntitySchema, field.Name)
		}
		if _, reserved := reservedFields[field.Name]; reserved {
			return fmt.Errorf("%w: field name %q is reserved", ErrInvalidEntitySchema, field.Name)
		}
		if _, duplicate := seen[field.Name]; duplicate {
			return fmt.Errorf("%w: duplicate field %q", ErrInvalidEntitySchema, field.Name)
		}
		seen[field.Name] = struct{}{}
		if len(field.Label) < 1 || len(field.Label) > 120 {
			return fmt.Errorf("%w: field %q label must be 1-120 characters", ErrInvalidEntitySchema, field.Name)
		}
		if _, supported := supportedFieldTypes[field.Type]; !supported {
			return fmt.Errorf("%w: unsupported type %q for field %q", ErrInvalidEntitySchema, field.Type, field.Name)
		}
		if err := validateFieldOptions(field); err != nil {
			return err
		}
		if err := validateDefaultValue(field); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldOptions(field model.FieldDefinition) error {
	requiresOptions := field.Type == "select" || field.Type == "multiselect"
	if requiresOptions && (len(field.Options) == 0 || len(field.Options) > 100) {
		return fmt.Errorf("%w: field %q requires 1-100 options", ErrInvalidEntitySchema, field.Name)
	}
	if !requiresOptions && len(field.Options) > 0 {
		return fmt.Errorf("%w: field %q type %q does not accept options", ErrInvalidEntitySchema, field.Name, field.Type)
	}
	seen := map[string]struct{}{}
	for _, option := range field.Options {
		value := strings.TrimSpace(option)
		if value == "" || len(value) > 120 {
			return fmt.Errorf("%w: field %q contains an invalid option", ErrInvalidEntitySchema, field.Name)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: field %q contains duplicate option %q", ErrInvalidEntitySchema, field.Name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDefaultValue(field model.FieldDefinition) error {
	if field.Default == nil {
		return nil
	}
	switch field.Type {
	case "number", "money":
		if _, ok := field.Default.(float64); !ok {
			return defaultTypeError(field, "number")
		}
	case "boolean":
		if _, ok := field.Default.(bool); !ok {
			return defaultTypeError(field, "boolean")
		}
	case "multiselect":
		values, ok := field.Default.([]any)
		if !ok {
			return defaultTypeError(field, "array")
		}
		for _, value := range values {
			text, ok := value.(string)
			if !ok || !contains(field.Options, text) {
				return fmt.Errorf("%w: field %q default contains an unknown option", ErrInvalidEntitySchema, field.Name)
			}
		}
	default:
		text, ok := field.Default.(string)
		if !ok {
			return defaultTypeError(field, "string")
		}
		if field.Type == "select" && !contains(field.Options, text) {
			return fmt.Errorf("%w: field %q default is not an option", ErrInvalidEntitySchema, field.Name)
		}
	}
	return nil
}
func defaultTypeError(field model.FieldDefinition, expected string) error {
	return fmt.Errorf("%w: field %q default must be %s", ErrInvalidEntitySchema, field.Name, expected)
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
