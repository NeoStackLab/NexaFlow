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

type DynamicRecordHandler struct{ service service.DynamicRecordService }

func NewDynamicRecordHandler(service service.DynamicRecordService) *DynamicRecordHandler {
	return &DynamicRecordHandler{service: service}
}

func (h *DynamicRecordHandler) Create(c *gin.Context) {
	var input model.WriteRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4201, "invalid record payload", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	record, err := h.service.Create(ctx, tenantID(c), c.Param("entityID"), actorID(c), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, record)
}
func (h *DynamicRecordHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "25"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	items, total, err := h.service.List(ctx, tenantID(c), c.Param("entityID"), page, pageSize)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"items": items, "total": total, "page": max(page, 1), "page_size": min(max(pageSize, 1), 100)})
}
func (h *DynamicRecordHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	record, err := h.service.Get(ctx, tenantID(c), c.Param("entityID"), c.Param("recordID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, record)
}
func (h *DynamicRecordHandler) Update(c *gin.Context) {
	var input model.WriteRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4201, "invalid record payload", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	record, err := h.service.Update(ctx, tenantID(c), c.Param("entityID"), c.Param("recordID"), actorID(c), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, record)
}
func (h *DynamicRecordHandler) Delete(c *gin.Context) {
	version, err := strconv.Atoi(c.Query("expected_version"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4202, "expected_version is required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.Delete(ctx, tenantID(c), c.Param("entityID"), c.Param("recordID"), version); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"archived": true})
}
func (h *DynamicRecordHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidRecord):
		httpx.Error(c, http.StatusBadRequest, 4203, strings.TrimPrefix(err.Error(), service.ErrInvalidRecord.Error()+": "), nil)
	case errors.Is(err, repository.ErrEntityNotFound):
		httpx.Error(c, http.StatusNotFound, 4204, "entity not found", nil)
	case errors.Is(err, repository.ErrRecordNotFound):
		httpx.Error(c, http.StatusNotFound, 4205, "record not found", nil)
	case errors.Is(err, repository.ErrRecordConflict):
		httpx.Error(c, http.StatusConflict, 4206, "record version conflict", nil)
	case errors.Is(err, service.ErrQuotaExceeded):
		httpx.Error(c, http.StatusPaymentRequired, 4207, "record quota exceeded", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 4299, "dynamic record operation failed", nil)
	}
}
func actorID(c *gin.Context) string {
	value, ok := c.Get(appmiddleware.AuthClaimsKey)
	if !ok {
		return ""
	}
	claims, ok := value.(*service.AccessClaims)
	if !ok {
		return ""
	}
	return claims.Subject
}
