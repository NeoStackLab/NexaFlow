package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
)

var ErrInvalidFormSubmission = errors.New("invalid form submission")

type FormSubmissionService interface {
	Submit(ctx context.Context, tenantID, formID, actorID string, input model.SubmitFormInput) (model.FormSubmission, error)
	List(ctx context.Context, tenantID, formID string, page, pageSize int) ([]model.FormSubmission, int64, error)
}

type formSubmissionService struct {
	forms      FormService
	records    DynamicRecordService
	repository repository.FormSubmissionRepository
}

func NewFormSubmissionService(forms FormService, records DynamicRecordService, repository repository.FormSubmissionRepository) FormSubmissionService {
	return &formSubmissionService{forms: forms, records: records, repository: repository}
}

func (s *formSubmissionService) Submit(ctx context.Context, tenantID, formID, actorID string, input model.SubmitFormInput) (model.FormSubmission, error) {
	if len(input.Values) == 0 {
		return model.FormSubmission{}, fmt.Errorf("%w: values are required", ErrInvalidFormSubmission)
	}
	form, err := s.forms.Get(ctx, tenantID, formID)
	if err != nil {
		return model.FormSubmission{}, err
	}
	if actorID == "" {
		actorID = "00000000-0000-0000-0000-000000000000"
	}
	record, err := s.records.Create(ctx, tenantID, form.EntityID, actorID, model.WriteRecordInput{Values: input.Values})
	if err != nil {
		return model.FormSubmission{}, err
	}
	submission, err := s.repository.Create(ctx, tenantID, formID, form.EntityID, record.ID, actorID, record.Values)
	if err != nil {
		// Record creation and submission metadata use separate bounded service
		// interfaces. Archive the record when the second write fails so a retry
		// cannot leave an accepted record without a submission audit row.
		_ = s.records.Delete(context.WithoutCancel(ctx), tenantID, form.EntityID, record.ID, record.Version)
		return model.FormSubmission{}, err
	}
	return submission, nil
}

func (s *formSubmissionService) List(ctx context.Context, tenantID, formID string, page, pageSize int) ([]model.FormSubmission, int64, error) {
	if _, err := s.forms.Get(ctx, tenantID, formID); err != nil {
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
	return s.repository.List(ctx, tenantID, formID, (page-1)*pageSize, pageSize)
}
