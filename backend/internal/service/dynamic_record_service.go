package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

var ErrInvalidRecord = errors.New("invalid dynamic record")

type DynamicRecordService interface {
	Create(ctx context.Context, tenantID, entityID, actorID string, input model.WriteRecordInput) (model.RecordView, error)
	List(ctx context.Context, tenantID, entityID string, page, pageSize int) ([]model.RecordView, int64, error)
	Get(ctx context.Context, tenantID, entityID, recordID string) (model.RecordView, error)
	Update(ctx context.Context, tenantID, entityID, recordID, actorID string, input model.WriteRecordInput) (model.RecordView, error)
	Delete(ctx context.Context, tenantID, entityID, recordID string, expectedVersion int) error
}

type dynamicRecordService struct {
	repository DynamicRecordRepository
	models     DynamicModelService
	meter      UsageMeter
}

// DynamicRecordRepository is owned by the service boundary so persistence only
// receives values that have already passed the active entity schema.
type DynamicRecordRepository interface {
	Create(ctx context.Context, tenantID, entityID, actorID string, values map[string]any) (model.RecordView, error)
	List(ctx context.Context, tenantID, entityID string, offset, limit int) ([]model.RecordView, int64, error)
	Get(ctx context.Context, tenantID, entityID, recordID string) (model.RecordView, error)
	Update(ctx context.Context, tenantID, entityID, recordID, actorID string, expectedVersion int, values map[string]any) (model.RecordView, error)
	Delete(ctx context.Context, tenantID, entityID, recordID string, expectedVersion int) error
}

func NewDynamicRecordService(repository DynamicRecordRepository, models DynamicModelService, meters ...UsageMeter) DynamicRecordService {
	var meter UsageMeter
	if len(meters) > 0 {
		meter = meters[0]
	}
	return &dynamicRecordService{repository: repository, models: models, meter: meter}
}

func (s *dynamicRecordService) Create(ctx context.Context, tenantID, entityID, actorID string, input model.WriteRecordInput) (model.RecordView, error) {
	schema, err := s.models.Get(ctx, tenantID, entityID)
	if err != nil {
		return model.RecordView{}, err
	}
	values, err := validateRecordValues(schema, input.Values)
	if err != nil {
		return model.RecordView{}, err
	}
	if s.meter != nil {
		if err := s.meter.Consume(ctx, tenantID, "records", 1); err != nil {
			return model.RecordView{}, err
		}
	}
	result, err := s.repository.Create(ctx, tenantID, entityID, actorID, values)
	if err != nil && s.meter != nil {
		_ = s.meter.Consume(context.WithoutCancel(ctx), tenantID, "records", -1)
	}
	return result, err
}
func (s *dynamicRecordService) List(ctx context.Context, tenantID, entityID string, page, pageSize int) ([]model.RecordView, int64, error) {
	if _, err := s.models.Get(ctx, tenantID, entityID); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.repository.List(ctx, tenantID, entityID, (page-1)*pageSize, pageSize)
}
func (s *dynamicRecordService) Get(ctx context.Context, tenantID, entityID, recordID string) (model.RecordView, error) {
	if _, err := s.models.Get(ctx, tenantID, entityID); err != nil {
		return model.RecordView{}, err
	}
	return s.repository.Get(ctx, tenantID, entityID, recordID)
}
func (s *dynamicRecordService) Update(ctx context.Context, tenantID, entityID, recordID, actorID string, input model.WriteRecordInput) (model.RecordView, error) {
	if input.ExpectedVersion < 1 {
		return model.RecordView{}, fmt.Errorf("%w: expected_version is required", ErrInvalidRecord)
	}
	schema, err := s.models.Get(ctx, tenantID, entityID)
	if err != nil {
		return model.RecordView{}, err
	}
	values, err := validateRecordValues(schema, input.Values)
	if err != nil {
		return model.RecordView{}, err
	}
	return s.repository.Update(ctx, tenantID, entityID, recordID, actorID, input.ExpectedVersion, values)
}
func (s *dynamicRecordService) Delete(ctx context.Context, tenantID, entityID, recordID string, expectedVersion int) error {
	if expectedVersion < 1 {
		return fmt.Errorf("%w: expected_version is required", ErrInvalidRecord)
	}
	if _, err := s.models.Get(ctx, tenantID, entityID); err != nil {
		return err
	}
	err := s.repository.Delete(ctx, tenantID, entityID, recordID, expectedVersion)
	if err == nil && s.meter != nil {
		_ = s.meter.Consume(context.WithoutCancel(ctx), tenantID, "records", -1)
	}
	return err
}

func validateRecordValues(schema model.EntityDefinition, raw map[string]any) (map[string]any, error) {
	values := make(map[string]any, len(schema.Fields))
	fields := make(map[string]model.FieldDefinition, len(schema.Fields))
	for _, field := range schema.Fields {
		fields[field.Name] = field
	}
	for name := range raw {
		if _, exists := fields[name]; !exists {
			return nil, fmt.Errorf("%w: unknown field %q", ErrInvalidRecord, name)
		}
	}
	for _, field := range schema.Fields {
		value, exists := raw[field.Name]
		if !exists || value == nil || value == "" {
			if field.Default != nil {
				values[field.Name] = field.Default
				continue
			}
			if field.Required {
				return nil, fmt.Errorf("%w: field %q is required", ErrInvalidRecord, field.Name)
			}
			continue
		}
		if err := validateRecordValue(field, value); err != nil {
			return nil, err
		}
		values[field.Name] = value
	}
	return values, nil
}

var moneyPattern = regexp.MustCompile(`^-?\d+(\.\d{1,4})?$`)

func validateRecordValue(field model.FieldDefinition, value any) error {
	invalid := func() error {
		return fmt.Errorf("%w: field %q has invalid %s value", ErrInvalidRecord, field.Name, field.Type)
	}
	switch field.Type {
	case "number":
		if _, ok := value.(float64); !ok {
			return invalid()
		}
	case "money":
		number, ok := value.(float64)
		if !ok || !moneyPattern.MatchString(fmt.Sprintf("%v", number)) {
			return invalid()
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return invalid()
		}
	case "date":
		text, ok := value.(string)
		if !ok {
			return invalid()
		}
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return invalid()
		}
	case "datetime":
		text, ok := value.(string)
		if !ok {
			return invalid()
		}
		if _, err := time.Parse(time.RFC3339, text); err != nil {
			return invalid()
		}
	case "select":
		text, ok := value.(string)
		if !ok || !contains(field.Options, text) {
			return invalid()
		}
	case "multiselect":
		items, ok := value.([]any)
		if !ok {
			return invalid()
		}
		for _, item := range items {
			text, ok := item.(string)
			if !ok || !contains(field.Options, text) {
				return invalid()
			}
		}
	case "email":
		text, ok := value.(string)
		if !ok {
			return invalid()
		}
		address, err := mail.ParseAddress(text)
		if err != nil || address.Address != text {
			return invalid()
		}
	case "url":
		text, ok := value.(string)
		if !ok {
			return invalid()
		}
		parsed, err := url.ParseRequestURI(text)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return invalid()
		}
	default:
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return invalid()
		}
	}
	return nil
}
