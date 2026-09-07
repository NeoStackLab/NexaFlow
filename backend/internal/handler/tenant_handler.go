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
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"github.com/gin-gonic/gin"
)

type TenantHandler struct{ service service.TenantService }

func NewTenantHandler(service service.TenantService) *TenantHandler {
	return &TenantHandler{service: service}
}

func (h *TenantHandler) Get(c *gin.Context) {
	h.withContext(c, func(ctx context.Context) (any, error) { return h.service.Get(ctx) })
}

func (h *TenantHandler) Update(c *gin.Context) {
	var input model.UpdateTenantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3201, "invalid tenant update", nil)
		return
	}
	h.withContext(c, func(ctx context.Context) (any, error) { return h.service.Update(ctx, input) })
}

func (h *TenantHandler) Suspend(c *gin.Context) {
	h.withContext(c, func(ctx context.Context) (any, error) { return h.service.Suspend(ctx) })
}

func (h *TenantHandler) Archive(c *gin.Context) {
	h.withContext(c, func(ctx context.Context) (any, error) { return h.service.Archive(ctx) })
}

func (h *TenantHandler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.Delete(ctx); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"deleted": true})
}

func (h *TenantHandler) Settings(c *gin.Context) {
	h.withContext(c, func(ctx context.Context) (any, error) { return h.service.Settings(ctx) })
}

func (h *TenantHandler) UpdateSettings(c *gin.Context) {
	var input model.UpdateTenantSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3202, "invalid tenant settings", nil)
		return
	}
	h.withContext(c, func(ctx context.Context) (any, error) { return h.service.UpdateSettings(ctx, input) })
}

func (h *TenantHandler) withContext(c *gin.Context, action func(context.Context) (any, error)) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := action(ctx)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}

func (h *TenantHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidTenantSettings):
		httpx.Error(c, http.StatusBadRequest, 3203, strings.TrimPrefix(err.Error(), service.ErrInvalidTenantSettings.Error()+": "), nil)
	case errors.Is(err, repository.ErrTenantVersionConflict):
		httpx.Error(c, http.StatusConflict, 3204, "tenant version conflict", nil)
	case errors.Is(err, repository.ErrCustomDomainConflict):
		httpx.Error(c, http.StatusConflict, 3208, "custom domain is already assigned", nil)
	case errors.Is(err, repository.ErrTenantLifecycle):
		httpx.Error(c, http.StatusConflict, 3205, "invalid tenant lifecycle transition", nil)
	case errors.Is(err, repository.ErrPrimaryTenantProtected):
		httpx.Error(c, http.StatusForbidden, 3206, "primary tenant lifecycle is protected", nil)
	case errors.Is(err, repository.ErrTenantNotFound):
		httpx.Error(c, http.StatusNotFound, 3207, "tenant not found", nil)
	case errors.Is(err, tenantcontext.ErrRequired), errors.Is(err, tenantcontext.ErrTenantMismatch):
		httpx.Error(c, http.StatusUnauthorized, 3002, "invalid tenant context", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 3200, "tenant operation failed", nil)
	}
}
