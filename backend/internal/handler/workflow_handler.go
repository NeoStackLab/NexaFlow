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

type WorkflowHandler struct{ service service.WorkflowService }

func NewWorkflowHandler(service service.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{service: service}
}
func (h *WorkflowHandler) Define(c *gin.Context) {
	var input model.DefineWorkflowInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4401, "invalid workflow definition", nil)
		return
	}
	input.ID = c.Param("workflowID")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := h.service.Define(ctx, tenantID(c), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	status := http.StatusOK
	if input.ID == "" {
		status = http.StatusCreated
	}
	httpx.Success(c, status, result)
}
func (h *WorkflowHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.service.List(ctx, tenantID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *WorkflowHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.service.Get(ctx, tenantID(c), c.Param("workflowID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *WorkflowHandler) Archive(c *gin.Context) {
	version, err := strconv.Atoi(c.Query("expected_version"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4402, "expected_version is required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.Archive(ctx, tenantID(c), c.Param("workflowID"), version); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"archived": true})
}
func (h *WorkflowHandler) Start(c *gin.Context) {
	var input model.StartWorkflowInput
	if err := c.ShouldBindJSON(&input); err != nil || input.RecordID == "" {
		httpx.Error(c, http.StatusBadRequest, 4403, "record_id is required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := h.service.Start(ctx, tenantID(c), c.Param("workflowID"), input.RecordID, actorID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, result)
}
func (h *WorkflowHandler) Instances(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.service.ListInstances(ctx, tenantID(c), c.Query("workflow_id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *WorkflowHandler) Act(c *gin.Context) {
	var input model.WorkflowActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4404, "invalid workflow action", nil)
		return
	}
	roles := []string{}
	if value, ok := c.Get(appmiddleware.AuthUserKey); ok {
		if user, ok := value.(model.AuthUser); ok {
			roles = user.Roles
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := h.service.Act(ctx, tenantID(c), c.Param("instanceID"), actorID(c), roles, input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *WorkflowHandler) Notifications(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.service.ListNotifications(ctx, tenantID(c), actorID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *WorkflowHandler) ReadNotification(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.ReadNotification(ctx, tenantID(c), actorID(c), c.Param("notificationID")); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"read": true})
}
func (h *WorkflowHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidWorkflow):
		httpx.Error(c, http.StatusBadRequest, 4405, strings.TrimPrefix(err.Error(), service.ErrInvalidWorkflow.Error()+": "), nil)
	case errors.Is(err, service.ErrWorkflowActionDenied):
		httpx.Error(c, http.StatusForbidden, 4406, "workflow action denied", nil)
	case errors.Is(err, repository.ErrWorkflowNotFound):
		httpx.Error(c, http.StatusNotFound, 4407, "workflow not found", nil)
	case errors.Is(err, repository.ErrWorkflowConflict):
		httpx.Error(c, http.StatusConflict, 4408, "workflow version conflict", nil)
	case errors.Is(err, repository.ErrWorkflowSlugUsed):
		httpx.Error(c, http.StatusConflict, 4409, "workflow slug already exists", nil)
	case errors.Is(err, repository.ErrInstanceNotFound):
		httpx.Error(c, http.StatusNotFound, 4410, "workflow instance not found", nil)
	case errors.Is(err, repository.ErrInstanceConflict):
		httpx.Error(c, http.StatusConflict, 4411, "workflow instance version conflict", nil)
	case errors.Is(err, repository.ErrRecordNotFound):
		httpx.Error(c, http.StatusNotFound, 4412, "record not found", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 4499, "workflow operation failed", nil)
	}
}
