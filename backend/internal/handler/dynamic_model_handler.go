package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	appmiddleware "github.com/NeoStackLab/NexaFlow/backend/internal/middleware"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type DynamicModelHandler struct{ service service.DynamicModelService }

func NewDynamicModelHandler(service service.DynamicModelService) *DynamicModelHandler {
	return &DynamicModelHandler{service: service}
}

func (h *DynamicModelHandler) Define(c *gin.Context) {
	var input model.DefineEntityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4101, "invalid entity definition", nil)
		return
	}
	input.ID = c.Param("entityID")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	definition, err := h.service.Define(ctx, tenantID(c), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	status := http.StatusOK
	if input.ID == "" {
		status = http.StatusCreated
	}
	httpx.Success(c, status, definition)
}
func (h *DynamicModelHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	definitions, err := h.service.List(ctx, tenantID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, definitions)
}
func (h *DynamicModelHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	definition, err := h.service.Get(ctx, tenantID(c), c.Param("entityID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, definition)
}
func (h *DynamicModelHandler) Archive(c *gin.Context) {
	version, err := strconv.Atoi(c.Query("expected_version"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4102, "expected_version is required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.Archive(ctx, tenantID(c), c.Param("entityID"), version); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"archived": true})
}

func tenantID(c *gin.Context) string {
	value, ok := c.Get(appmiddleware.AuthClaimsKey)
	if !ok {
		return ""
	}
	claims, ok := value.(*service.AccessClaims)
	if !ok {
		return ""
	}
	return claims.TenantID
}
func (h *DynamicModelHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidEntitySchema):
		httpx.Error(c, http.StatusBadRequest, 4103, strings.TrimPrefix(err.Error(), service.ErrInvalidEntitySchema.Error()+": "), nil)
	case errors.Is(err, repository.ErrEntityNotFound):
		httpx.Error(c, http.StatusNotFound, 4104, "entity not found", nil)
	case errors.Is(err, repository.ErrEntityConflict):
		httpx.Error(c, http.StatusConflict, 4105, "entity schema version conflict", nil)
	case errors.Is(err, repository.ErrEntitySlugUsed):
		httpx.Error(c, http.StatusConflict, 4106, "entity slug already exists in this tenant", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 4199, "dynamic model operation failed", nil)
	}
}
