package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.String("request_id", c.GetString(RequestIDKey)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int("bytes", c.Writer.Size()),
			zap.Duration("latency", time.Since(startedAt)),
			zap.String("client_ip", c.ClientIP()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
		}
		if spanContext := trace.SpanContextFromContext(c.Request.Context()); spanContext.IsValid() {
			fields = append(fields, zap.String("trace_id", spanContext.TraceID().String()), zap.String("span_id", spanContext.SpanID().String()))
		}
		if value, ok := c.Get(AuthUserKey); ok {
			if user, ok := value.(model.AuthUser); ok {
				fields = append(fields, zap.String("user_id", user.ID), zap.String("tenant_id", user.ActiveTenantID))
			}
		}
		log.Info("http_request", fields...)
	}
}

// RequestAudit records authenticated mutating requests without persisting
// request bodies. It is deliberately best-effort: a telemetry outage must not
// turn a successful business mutation into a failed HTTP response.
func RequestAudit(audit service.AuditService, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if audit == nil || c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			return
		}
		value, ok := c.Get(AuthUserKey)
		user, ok := value.(model.AuthUser)
		if !ok || user.ActiveTenantID == "" {
			return
		}
		metadata, _ := json.Marshal(map[string]any{"status_code": c.Writer.Status()})
		path := c.Request.URL.Path
		if len(path) > 500 {
			path = path[:500]
		}
		resource := path
		if len(resource) > 160 {
			resource = resource[:160]
		}
		event := model.AuditEvent{TenantID: user.ActiveTenantID, ActorID: user.ID, Action: "http.request", Resource: resource, Method: c.Request.Method, Path: path, StatusCode: c.Writer.Status(), RequestID: c.GetString(RequestIDKey), IPAddress: c.ClientIP(), Metadata: metadata}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Second)
		defer cancel()
		if err := audit.Record(ctx, event); err != nil {
			log.Warn("audit_event_write_failed", zap.String("path", c.Request.URL.Path), zap.Error(err))
		}
	}
}
