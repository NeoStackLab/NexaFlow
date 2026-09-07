package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct{ service service.AuditService }

func NewAuditHandler(auditService service.AuditService) *AuditHandler {
	return &AuditHandler{service: auditService}
}

func (h *AuditHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	events, err := h.service.List(ctx, tenantID(c), limit)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, 4801, "audit events unavailable", nil)
		return
	}
	httpx.Success(c, http.StatusOK, events)
}
