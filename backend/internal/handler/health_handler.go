package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	service service.HealthService
}

func NewHealthHandler(service service.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	httpx.Success(c, http.StatusOK, h.service.Liveness())
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := c.Request.Context(), func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
	}
	defer cancel()

	status, err := h.service.Readiness(ctx)
	if err != nil {
		httpx.Error(c, http.StatusServiceUnavailable, 1001, "service dependencies unavailable", status)
		return
	}
	httpx.Success(c, http.StatusOK, status)
}
