package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"github.com/gin-gonic/gin"
)

const AuthClaimsKey = "auth_claims"
const AuthUserKey = "auth_user"
const TenantContextKey = "tenant_context"

func Authenticate(auth service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			httpx.Error(c, http.StatusUnauthorized, 3001, "authentication required", nil)
			c.Abort()
			return
		}
		claims, err := auth.ParseAccessToken(strings.TrimSpace(parts[1]))
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, 3002, "invalid or expired access token", nil)
			c.Abort()
			return
		}
		if requestedTenant := strings.TrimSpace(c.GetHeader("X-Tenant-ID")); requestedTenant != "" && requestedTenant != claims.TenantID {
			httpx.Error(c, http.StatusForbidden, 3004, "tenant context does not match access token", nil)
			c.Abort()
			return
		}
		provisional := tenantcontext.With(c.Request.Context(), tenantcontext.Scope{TenantID: claims.TenantID, UserID: claims.Subject, Roles: claims.Roles, Permissions: claims.Permissions})
		ctx, cancel := context.WithTimeout(provisional, 5*time.Second)
		defer cancel()
		user, err := auth.Me(ctx, claims.Subject, claims.TenantID)
		if err != nil {
			if errors.Is(err, service.ErrAccountDisabled) {
				httpx.Error(c, http.StatusForbidden, 3107, "account is disabled", nil)
			} else {
				httpx.Error(c, http.StatusUnauthorized, 3005, "tenant membership is no longer active", nil)
			}
			c.Abort()
			return
		}
		c.Set(AuthClaimsKey, claims)
		c.Set(AuthUserKey, user)
		c.Next()
	}
}

// TenantContext promotes the verified authentication result into the standard
// context.Context used by every service and repository call.
func TenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(AuthUserKey)
		user, valid := value.(model.AuthUser)
		if !ok || !valid || user.ID == "" || user.ActiveTenantID == "" {
			httpx.Error(c, http.StatusUnauthorized, 3002, "missing authenticated tenant context", nil)
			c.Abort()
			return
		}
		primaryRole := ""
		if len(user.Roles) > 0 {
			primaryRole = user.Roles[0]
		}
		scope := tenantcontext.Scope{TenantID: user.ActiveTenantID, UserID: user.ID, PrimaryRole: primaryRole, Roles: user.Roles, Permissions: user.Permissions}
		c.Set(TenantContextKey, scope)
		c.Request = c.Request.WithContext(tenantcontext.With(c.Request.Context(), scope))
		c.Next()
	}
}

func RequirePermission(_ service.AuthService, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := tenantcontext.Require(c.Request.Context()); err != nil {
			httpx.Error(c, http.StatusUnauthorized, 3002, "missing authenticated user context", nil)
			c.Abort()
			return
		}
		if tenantcontext.HasPermission(c.Request.Context(), permission) {
			c.Next()
			return
		}
		httpx.Error(c, http.StatusForbidden, 3003, "permission denied", nil)
		c.Abort()
	}
}
