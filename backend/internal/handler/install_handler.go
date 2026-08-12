package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type InstallHandler struct{ service service.InstallService }

func NewInstallHandler(service service.InstallService) *InstallHandler {
	return &InstallHandler{service: service}
}

func (h *InstallHandler) Status(c *gin.Context) {
	httpx.Success(c, http.StatusOK, h.service.Status())
}

func (h *InstallHandler) Environment(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	httpx.Success(c, http.StatusOK, h.service.Environment(ctx))
}

func (h *InstallHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	httpx.Success(c, http.StatusOK, h.service.Readiness(ctx))
}

func (h *InstallHandler) Complete(c *gin.Context) {
	var input model.CompleteInstallationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 2002, "invalid installation request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := h.service.Complete(ctx, input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, result)
}

func (h *InstallHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAlreadyInstalled):
		httpx.Error(c, http.StatusConflict, 2003, "NexaFlow is already installed", nil)
	case errors.Is(err, service.ErrInvalidInstall):
		httpx.Error(c, http.StatusBadRequest, 2004, err.Error(), nil)
	default:
		httpx.Error(c, http.StatusUnprocessableEntity, 2005, "installation operation failed", gin.H{"detail": err.Error()})
	}
}
