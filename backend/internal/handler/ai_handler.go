package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	appmiddleware "github.com/NeoStackLab/NexaFlow/backend/internal/middleware"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	knowledge service.KnowledgeService
	agent     service.AgentService
	ingestion service.KnowledgeIngestionService
}

func NewAIHandler(knowledge service.KnowledgeService, agent service.AgentService, ingestion ...service.KnowledgeIngestionService) *AIHandler {
	var jobs service.KnowledgeIngestionService
	if len(ingestion) > 0 {
		jobs = ingestion[0]
	}
	return &AIHandler{knowledge: knowledge, agent: agent, ingestion: jobs}
}
func (h *AIHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (20<<20)+1)
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4501, "knowledge file is required", nil)
		return
	}
	source, err := file.Open()
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4502, "open uploaded file failed", nil)
		return
	}
	defer source.Close()
	if h.ingestion != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		job, err := h.ingestion.Enqueue(ctx, tenantID(c), actorID(c), file.Filename, file.Header.Get("Content-Type"), source)
		if err != nil {
			h.writeError(c, err)
			return
		}
		httpx.Success(c, http.StatusAccepted, job)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	result, err := h.knowledge.Ingest(ctx, tenantID(c), actorID(c), file.Filename, file.Header.Get("Content-Type"), file.Size, source)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, result)
}
func (h *AIHandler) IngestionJobs(c *gin.Context) {
	if h.ingestion == nil {
		httpx.Success(c, http.StatusOK, []model.KnowledgeIngestionJob{})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.ingestion.List(ctx, tenantID(c), limit)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *AIHandler) Documents(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.knowledge.List(ctx, tenantID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *AIHandler) DeleteDocument(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if err := h.knowledge.Delete(ctx, tenantID(c), c.Param("documentID")); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"archived": true})
}
func (h *AIHandler) Search(c *gin.Context) {
	var input struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4503, "invalid search request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	defer cancel()
	result, err := h.knowledge.Search(ctx, tenantID(c), input.Query, input.Limit)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *AIHandler) Ask(c *gin.Context) {
	var input model.AIAskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 4504, "invalid AI request", nil)
		return
	}
	permissions := []string{}
	if value, ok := c.Get(appmiddleware.AuthUserKey); ok {
		if user, ok := value.(model.AuthUser); ok {
			permissions = user.Permissions
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	result, err := h.agent.Ask(ctx, tenantID(c), actorID(c), permissions, input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *AIHandler) Conversations(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.agent.ListConversations(ctx, tenantID(c), actorID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *AIHandler) Messages(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.agent.ListMessages(ctx, tenantID(c), actorID(c), c.Param("conversationID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, result)
}
func (h *AIHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAIProviderUnavailable):
		httpx.Error(c, http.StatusServiceUnavailable, 4505, "AI provider is not configured", nil)
	case errors.Is(err, service.ErrInvalidKnowledgeDocument), errors.Is(err, service.ErrInvalidAIRequest):
		httpx.Error(c, http.StatusBadRequest, 4506, err.Error(), nil)
	case errors.Is(err, repository.ErrKnowledgeNotFound):
		httpx.Error(c, http.StatusNotFound, 4507, "knowledge document not found", nil)
	case errors.Is(err, repository.ErrConversationNotFound):
		httpx.Error(c, http.StatusNotFound, 4508, "AI conversation not found", nil)
	case errors.Is(err, service.ErrQuotaExceeded):
		httpx.Error(c, http.StatusPaymentRequired, 4509, "plan quota exceeded", nil)
	default:
		httpx.Error(c, http.StatusBadGateway, 4599, "AI operation failed", nil)
	}
}
