package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type auditServiceStub struct{ events []model.AuditEvent }

func (s *auditServiceStub) Record(_ context.Context, event model.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *auditServiceStub) List(context.Context, string, int) ([]model.AuditEvent, error) {
	return s.events, nil
}

func TestRequestAuditRecordsAuthenticatedMutationsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &auditServiceStub{}
	router := gin.New()
	router.Use(RequestID(), RequestAudit(stub, zap.NewNop()))
	router.POST("/api/v1/entities", func(c *gin.Context) {
		c.Set(AuthUserKey, model.AuthUser{ID: "actor-1", ActiveTenantID: "tenant-1"})
		c.Status(http.StatusCreated)
	})
	router.GET("/api/v1/entities", func(c *gin.Context) {
		c.Set(AuthUserKey, model.AuthUser{ID: "actor-1", ActiveTenantID: "tenant-1"})
		c.Status(http.StatusOK)
	})

	post := httptest.NewRecorder()
	router.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/v1/entities", nil))
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/entities", nil))

	if len(stub.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(stub.events))
	}
	event := stub.events[0]
	if event.TenantID != "tenant-1" || event.ActorID != "actor-1" || event.Method != http.MethodPost || event.StatusCode != http.StatusCreated || event.RequestID == "" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
}
