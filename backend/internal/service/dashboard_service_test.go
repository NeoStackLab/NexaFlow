package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type dashboardRepositoryStub struct{ saved bool }

func (*dashboardRepositoryStub) Get(context.Context, string) (model.DashboardView, error) {
	return model.DashboardView{}, nil
}
func (s *dashboardRepositoryStub) Save(_ context.Context, _, _ string, input model.SaveDashboardInput) (model.DashboardView, error) {
	s.saved = true
	return model.DashboardView{Widgets: input.Widgets, Version: 1}, nil
}
func (*dashboardRepositoryStub) Values(context.Context, string, []model.DashboardWidget) (map[string]float64, error) {
	return map[string]float64{}, nil
}

func TestDashboardRejectsInvalidSumField(t *testing.T) {
	repository := &dashboardRepositoryStub{}
	models := &modelServiceStub{schema: model.EntityDefinition{ID: "entity-1", Fields: []model.FieldDefinition{{Name: "name", Type: "string"}}}}
	service := NewDashboardService(repository, models)
	_, err := service.Save(context.Background(), "tenant-a", "user-1", model.SaveDashboardInput{Widgets: []model.DashboardWidget{{ID: "revenue", Type: "sum", Title: "Revenue", EntityID: "entity-1", Field: "name", Width: 1}}})
	if !errors.Is(err, ErrInvalidDashboard) || repository.saved {
		t.Fatalf("error=%v saved=%v", err, repository.saved)
	}
}

func TestDashboardAcceptsBoundedRecordWidget(t *testing.T) {
	repository := &dashboardRepositoryStub{}
	models := &modelServiceStub{schema: model.EntityDefinition{ID: "entity-1"}}
	service := NewDashboardService(repository, models)
	result, err := service.Save(context.Background(), "tenant-a", "user-1", model.SaveDashboardInput{Widgets: []model.DashboardWidget{{ID: "orders", Type: "records", Title: "Orders", EntityID: "entity-1", Width: 2}}})
	if err != nil || !repository.saved || result.Version != 1 {
		t.Fatalf("result=%#v error=%v saved=%v", result, err, repository.saved)
	}
}
