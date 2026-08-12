package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	appmiddleware "github.com/NeoStackLab/NexaFlow/backend/internal/middleware"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ service service.AuthService }

func NewAuthHandler(service service.AuthService) *AuthHandler { return &AuthHandler{service: service} }

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TenantID string `json:"tenant_id"`
}
type tokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}
type permissionsRequest struct {
	Permissions []string `json:"permissions"`
}
type rolesRequest struct {
	Roles []string `json:"roles"`
}
type switchTenantRequest struct {
	TenantID     string `json:"tenant_id"`
	RefreshToken string `json:"refresh_token"`
}
type createTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3101, "invalid registration request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	user, err := h.service.Register(ctx, model.RegisterInput{Username: request.Username, Email: request.Email, Password: request.Password})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3102, "invalid login request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	tokens, err := h.service.Login(ctx, model.LoginInput{Email: request.Email, Password: request.Password, TenantID: request.TenantID, UserAgent: c.Request.UserAgent(), IPAddress: c.ClientIP()})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, tokens)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var request tokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3103, "invalid refresh request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	tokens, err := h.service.Refresh(ctx, model.RefreshInput{RefreshToken: request.RefreshToken, UserAgent: c.Request.UserAgent(), IPAddress: c.ClientIP()})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var request tokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3104, "invalid logout request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.Logout(ctx, model.LogoutInput{RefreshToken: request.RefreshToken}); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"revoked": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	httpx.Success(c, http.StatusOK, user)
}

func (h *AuthHandler) Sessions(c *gin.Context) {
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	sessions, err := h.service.Sessions(ctx, claims.Subject, claims.TenantID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, sessions)
}

func (h *AuthHandler) Menu(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	httpx.Success(c, http.StatusOK, h.service.Menu(user))
}

func (h *AuthHandler) RevokeSession(c *gin.Context) {
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.RevokeSession(ctx, claims.Subject, claims.TenantID, c.Param("sessionID")); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"revoked": true})
}
func (h *AuthHandler) Roles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	roles, err := h.service.Roles(ctx, claims.TenantID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, roles)
}
func (h *AuthHandler) Permissions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	permissions, err := h.service.Permissions(ctx)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, permissions)
}
func (h *AuthHandler) SetRolePermissions(c *gin.Context) {
	var request permissionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3111, "invalid permission assignment", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	if err := h.service.SetRolePermissions(ctx, claims.TenantID, c.Param("roleID"), request.Permissions); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"updated": true})
}
func (h *AuthHandler) Users(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	users, err := h.service.Users(ctx, claims.TenantID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, users)
}
func (h *AuthHandler) SetUserRoles(c *gin.Context) {
	var request rolesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3112, "invalid role assignment", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	if err := h.service.SetUserRoles(ctx, claims.TenantID, c.Param("userID"), request.Roles); err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"updated": true})
}

func (h *AuthHandler) currentUser(c *gin.Context) (model.AuthUser, bool) {
	if value, exists := c.Get(appmiddleware.AuthUserKey); exists {
		if user, ok := value.(model.AuthUser); ok {
			return user, true
		}
	}
	claims, ok := authClaims(c)
	if !ok {
		return model.AuthUser{}, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	user, err := h.service.Me(ctx, claims.Subject, claims.TenantID)
	if err != nil {
		h.writeError(c, err)
		return model.AuthUser{}, false
	}
	return user, true
}

func authClaims(c *gin.Context) (*service.AccessClaims, bool) {
	value, ok := c.Get(appmiddleware.AuthClaimsKey)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, 3001, "authentication required", nil)
		return nil, false
	}
	claims, ok := value.(*service.AccessClaims)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, 3002, "invalid access token context", nil)
		return nil, false
	}
	return claims, true
}

func (h *AuthHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		httpx.Error(c, http.StatusUnauthorized, 3105, "invalid email or password", nil)
	case errors.Is(err, service.ErrInvalidToken), errors.Is(err, repository.ErrSessionNotFound):
		httpx.Error(c, http.StatusUnauthorized, 3106, "invalid or expired refresh token", nil)
	case errors.Is(err, service.ErrAccountDisabled):
		httpx.Error(c, http.StatusForbidden, 3107, "account is disabled", nil)
	case errors.Is(err, service.ErrInvalidAuthInput):
		httpx.Error(c, http.StatusBadRequest, 3108, strings.TrimPrefix(err.Error(), service.ErrInvalidAuthInput.Error()+": "), nil)
	case errors.Is(err, repository.ErrUserExists):
		httpx.Error(c, http.StatusConflict, 3109, "username or email already exists", nil)
	case errors.Is(err, service.ErrTenantAccess), errors.Is(err, repository.ErrTenantAccess):
		httpx.Error(c, http.StatusForbidden, 3114, "tenant access denied", nil)
	default:
		httpx.Error(c, http.StatusInternalServerError, 3110, "authentication operation failed", nil)
	}
}

func (h *AuthHandler) Tenants(c *gin.Context) {
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	tenants, err := h.service.Tenants(ctx, claims.Subject)
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, tenants)
}
func (h *AuthHandler) CreateTenant(c *gin.Context) {
	claims, ok := authClaims(c)
	if !ok {
		return
	}
	var request createTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3115, "invalid tenant request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	tenant, err := h.service.CreateTenant(ctx, claims.Subject, model.CreateTenantInput{Name: request.Name, Slug: request.Slug})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, tenant)
}
func (h *AuthHandler) SwitchTenant(c *gin.Context) {
	var request switchTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, 3116, "invalid tenant switch request", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	pair, err := h.service.SwitchTenant(ctx, model.SwitchTenantInput{TenantID: request.TenantID, RefreshToken: request.RefreshToken, UserAgent: c.Request.UserAgent(), IPAddress: c.ClientIP()})
	if err != nil {
		h.writeError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, pair)
}
