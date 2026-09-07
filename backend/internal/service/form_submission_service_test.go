package service

import (
	"context"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type formSubmissionFormsStub struct{}

func (*formSubmissionFormsStub) Define(context.Context, string, model.DefineFormInput) (model.FormDefinition, error) {
	return model.FormDefinition{}, nil
}
func (*formSubmissionFormsStub) List(context.Context, string, string) ([]model.FormDefinition, error) {
	return nil, nil
}
func (*formSubmissionFormsStub) Get(context.Context, string, string) (model.FormDefinition, error) {
	return model.FormDefinition{ID: "form-1", EntityID: "entity-1"}, nil
}
func (*formSubmissionFormsStub) Archive(context.Context, string, string, int) error { return nil }

type formSubmissionRecordsStub struct {
	actorID string
	values  map[string]any
}

func (s *formSubmissionRecordsStub) Create(_ context.Context, _, _, actorID string, input model.WriteRecordInput) (model.RecordView, error) {
	s.actorID, s.values = actorID, input.Values
	return model.RecordView{ID: "record-1", Values: input.Values}, nil
}
func (*formSubmissionRecordsStub) List(context.Context, string, string, int, int) ([]model.RecordView, int64, error) {
	return nil, 0, nil
}
func (*formSubmissionRecordsStub) Get(context.Context, string, string, string) (model.RecordView, error) {
	return model.RecordView{}, nil
}
func (*formSubmissionRecordsStub) Update(context.Context, string, string, string, string, model.WriteRecordInput) (model.RecordView, error) {
	return model.RecordView{}, nil
}
func (*formSubmissionRecordsStub) Delete(context.Context, string, string, string, int) error {
	return nil
}

type formSubmissionRepositoryStub struct {
	formID   string
	entityID string
	recordID string
}

func (s *formSubmissionRepositoryStub) Create(_ context.Context, _, formID, entityID, recordID, _ string, _ map[string]any) (model.FormSubmission, error) {
	s.formID, s.entityID, s.recordID = formID, entityID, recordID
	return model.FormSubmission{ID: "submission-1", FormID: formID, RecordID: recordID}, nil
}
func (*formSubmissionRepositoryStub) List(context.Context, string, string, int, int) ([]model.FormSubmission, int64, error) {
	return nil, 0, nil
}

func TestFormSubmissionServiceValidatesThroughRecordService(t *testing.T) {
	records := &formSubmissionRecordsStub{}
	repository := &formSubmissionRepositoryStub{}
	service := NewFormSubmissionService(&formSubmissionFormsStub{}, records, repository)
	result, err := service.Submit(context.Background(), "tenant-1", "form-1", "user-1", model.SubmitFormInput{Values: map[string]any{"name": "Acme"}})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if result.ID != "submission-1" || records.actorID != "user-1" || repository.entityID != "entity-1" || repository.recordID != "record-1" {
		t.Fatalf("result=%#v actor=%q repository=%#v", result, records.actorID, repository)
	}
}

func TestFormSubmissionServiceRejectsEmptyValues(t *testing.T) {
	service := NewFormSubmissionService(&formSubmissionFormsStub{}, &formSubmissionRecordsStub{}, &formSubmissionRepositoryStub{})
	if _, err := service.Submit(context.Background(), "tenant-1", "form-1", "user-1", model.SubmitFormInput{}); err == nil {
		t.Fatal("expected empty submission to fail")
	}
}
