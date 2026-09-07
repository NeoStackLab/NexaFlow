package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"github.com/gin-gonic/gin"
)

func TestTenantContextPropagatesVerifiedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(AuthUserKey, model.AuthUser{ID: "user-1", ActiveTenantID: "tenant-1", Roles: []string{"admin"}, Permissions: []string{"tenant.view"}})
		c.Next()
	}, TenantContext())
	router.GET("/", func(c *gin.Context) {
		scope, err := tenantcontext.Require(c.Request.Context())
		if err != nil || scope.TenantID != "tenant-1" || scope.UserID != "user-1" || !tenantcontext.HasPermission(c.Request.Context(), "tenant.view") {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
}

func TestTenantContextFailsClosedWithoutVerifiedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TenantContext())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}
}
