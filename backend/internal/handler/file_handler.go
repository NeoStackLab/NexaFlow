package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type FileHandler struct{ service service.FileService }

func NewFileHandler(fileService service.FileService) *FileHandler {
	return &FileHandler{service: fileService}
}

func (h *FileHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4701, "file is required", nil)
		return
	}
	source, err := file.Open()
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, 4702, "open uploaded file failed", nil)
		return
	}
	defer source.Close()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	asset, err := h.service.Upload(ctx, tenantID(c), actorID(c), file.Filename, file.Header.Get("Content-Type"), file.Size, source)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, asset)
}

func (h *FileHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	assets, err := h.service.List(ctx, tenantID(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, assets)
}

func (h *FileHandler) Download(c *gin.Context) {
	asset, reader, err := h.service.Open(c.Request.Context(), tenantID(c), c.Param("fileID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(asset.Name, `"`, "")+`"`)
	c.DataFromReader(http.StatusOK, asset.Size, asset.ContentType, reader, nil)
}

func (h *FileHandler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	if err := h.service.Delete(ctx, tenantID(c), c.Param("fileID")); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"deleted": true})
}

func (h *FileHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidFile):
		httpx.Error(c, http.StatusBadRequest, 4703, strings.TrimPrefix(err.Error(), service.ErrInvalidFile.Error()+": "), nil)
	case errors.Is(err, repository.ErrFileNotFound):
		httpx.Error(c, http.StatusNotFound, 4704, "file not found", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 4799, "file operation failed", nil)
	}
}
