package api

import (
	"net/http"

	"github.com/NeoStackLab/NexaFlow/backend/internal/handler"
	appmiddleware "github.com/NeoStackLab/NexaFlow/backend/internal/middleware"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.Config, log *zap.Logger, healthHandler *handler.HealthHandler) *gin.Engine {
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(
		appmiddleware.RequestID(),
		appmiddleware.RequestLogger(log),
		appmiddleware.Recovery(log),
		appmiddleware.CORS(cfg.CORS.AllowedOrigins),
	)

	router.GET("/health/live", healthHandler.Liveness)
	router.GET("/health/ready", healthHandler.Readiness)

	v1 := router.Group("/api/v1")
	v1.GET("/health", healthHandler.Readiness)

	router.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, 4004, "resource not found", nil)
	})

	return router
}
