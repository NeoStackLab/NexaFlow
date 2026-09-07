package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct{ service service.DashboardService }

func NewDashboardHandler(dashboardService service.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: dashboardService}
}

func (h *DashboardHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	view, err := h.service.Get(ctx, tenantID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, view)
}

func (h *DashboardHandler) Save(c *gin.Context) {
	var input model.SaveDashboardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4801, "invalid dashboard definition", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	view, err := h.service.Save(ctx, tenantID(c), actorID(c), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, view)
}

func (h *DashboardHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidDashboard):
		httpx.Error(c, http.StatusBadRequest, 4802, strings.TrimPrefix(err.Error(), service.ErrInvalidDashboard.Error()+": "), nil)
	case errors.Is(err, repository.ErrEntityNotFound):
		httpx.Error(c, http.StatusNotFound, 4803, "entity not found", nil)
	case errors.Is(err, repository.ErrDashboardConflict):
		httpx.Error(c, http.StatusConflict, 4804, "dashboard version conflict", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 4899, "dashboard operation failed", nil)
	}
}
