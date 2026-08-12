package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

const AuthClaimsKey = "auth_claims"
const AuthUserKey = "auth_user"

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
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		user, err := auth.Me(ctx, claims.Subject, claims.TenantID)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, 3005, "tenant membership is no longer active", nil)
			c.Abort()
			return
		}
		c.Set(AuthClaimsKey, claims)
		c.Set(AuthUserKey, user)
		c.Next()
	}
}

func RequirePermission(_ service.AuthService, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, ok := c.Get(AuthUserKey)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, 3002, "missing authenticated user context", nil)
			c.Abort()
			return
		}
		user, ok := userValue.(model.AuthUser)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, 3002, "invalid authenticated user context", nil)
			c.Abort()
			return
		}
		for _, granted := range user.Permissions {
			if granted == permission {
				c.Next()
				return
			}
		}
		httpx.Error(c, http.StatusForbidden, 3003, "permission denied", nil)
		c.Abort()
	}
}
