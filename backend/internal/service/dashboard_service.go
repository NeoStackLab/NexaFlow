package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

var ErrInvalidDashboard = errors.New("invalid dashboard")

type DashboardRepository interface {
	Get(context.Context, string) (model.DashboardView, error)
	Save(context.Context, string, string, model.SaveDashboardInput) (model.DashboardView, error)
	Values(context.Context, string, []model.DashboardWidget) (map[string]float64, error)
}

type DashboardService interface {
	Get(context.Context, string) (model.DashboardView, error)
	Save(context.Context, string, string, model.SaveDashboardInput) (model.DashboardView, error)
}

type dashboardService struct {
	repository DashboardRepository
	models     DynamicModelService
}

func NewDashboardService(repository DashboardRepository, models DynamicModelService) DashboardService {
	return &dashboardService{repository: repository, models: models}
}

func (s *dashboardService) Get(ctx context.Context, tenantID string) (model.DashboardView, error) {
	view, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		return view, err
	}
	view.Values, err = s.repository.Values(ctx, tenantID, view.Widgets)
	return view, err
}

func (s *dashboardService) Save(ctx context.Context, tenantID, actorID string, input model.SaveDashboardInput) (model.DashboardView, error) {
	if len(input.Widgets) < 1 || len(input.Widgets) > 12 {
		return model.DashboardView{}, fmt.Errorf("%w: dashboard requires 1-12 widgets", ErrInvalidDashboard)
	}
	seen := map[string]bool{}
	for index := range input.Widgets {
		widget := &input.Widgets[index]
		widget.ID, widget.Title = strings.TrimSpace(widget.ID), strings.TrimSpace(widget.Title)
		if widget.ID == "" || len(widget.ID) > 80 || seen[widget.ID] || widget.Title == "" || len(widget.Title) > 100 || widget.Width < 1 || widget.Width > 3 {
			return model.DashboardView{}, fmt.Errorf("%w: widget metadata is invalid", ErrInvalidDashboard)
		}
		seen[widget.ID] = true
		switch widget.Type {
		case "users", "files":
			widget.EntityID, widget.Field = "", ""
		case "records", "sum":
			entity, err := s.models.Get(ctx, tenantID, widget.EntityID)
			if err != nil {
				return model.DashboardView{}, err
			}
			if widget.Type == "sum" && !numericDashboardField(entity, widget.Field) {
				return model.DashboardView{}, fmt.Errorf("%w: sum requires a number or money field", ErrInvalidDashboard)
			}
		default:
			return model.DashboardView{}, fmt.Errorf("%w: unsupported widget type", ErrInvalidDashboard)
		}
	}
	view, err := s.repository.Save(ctx, tenantID, actorID, input)
	if err != nil {
		return view, err
	}
	view.Values, err = s.repository.Values(ctx, tenantID, view.Widgets)
	return view, err
}

func numericDashboardField(entity model.EntityDefinition, name string) bool {
	for _, field := range entity.Fields {
		if field.Name == name && (field.Type == "number" || field.Type == "money") {
			return true
		}
	}
	return false
}
