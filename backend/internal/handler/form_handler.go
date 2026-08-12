package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type FormHandler struct{ service service.FormService }

func NewFormHandler(service service.FormService) *FormHandler { return &FormHandler{service: service} }
func (h *FormHandler) Define(c *gin.Context) {
	var input model.DefineFormInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4301, "invalid form definition", nil)
		return
	}
	input.ID = c.Param("formID")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	form, err := h.service.Define(ctx, tenantID(c), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	status := http.StatusOK
	if input.ID == "" {
		status = http.StatusCreated
	}
	httpx.Success(c, status, form)
}
func (h *FormHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	forms, err := h.service.List(ctx, tenantID(c), c.Query("entity_id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, forms)
}
func (h *FormHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	form, err := h.service.Get(ctx, tenantID(c), c.Param("formID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, form)
}
func (h *FormHandler) Archive(c *gin.Context) {
	version, err := strconv.Atoi(c.Query("expected_version"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4302, "expected_version is required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.Archive(ctx, tenantID(c), c.Param("formID"), version); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"archived": true})
}
func (h *FormHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidForm):
		httpx.Error(c, http.StatusBadRequest, 4303, strings.TrimPrefix(err.Error(), service.ErrInvalidForm.Error()+": "), nil)
	case errors.Is(err, repository.ErrEntityNotFound):
		httpx.Error(c, http.StatusNotFound, 4304, "entity not found", nil)
	case errors.Is(err, repository.ErrFormNotFound):
		httpx.Error(c, http.StatusNotFound, 4305, "form not found", nil)
	case errors.Is(err, repository.ErrFormConflict):
		httpx.Error(c, http.StatusConflict, 4306, "form version conflict", nil)
	case errors.Is(err, repository.ErrFormSlugUsed):
		httpx.Error(c, http.StatusConflict, 4307, "form slug already exists in this tenant", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 4399, "form operation failed", nil)
	}
}
