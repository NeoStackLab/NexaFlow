package middleware

import (
	"net/http"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error("panic_recovered",
			zap.Any("panic", recovered),
			zap.String("request_id", c.GetString(RequestIDKey)),
			zap.Stack("stack"),
		)
		httpx.Error(c, http.StatusInternalServerError, 5000, "internal server error", nil)
		c.Abort()
	})
}
