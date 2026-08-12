package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidAuthInput   = errors.New("invalid authentication input")
	ErrInvalidToken       = errors.New("invalid token")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrTenantAccess       = errors.New("tenant access denied")
)

var tenantSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,78}[a-z0-9]$`)

type AccessClaims struct {
	Email       string   `json:"email"`
	TenantID    string   `json:"tenant_id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

type AuthService interface {
	Register(ctx context.Context, input model.RegisterInput) (model.AuthUser, error)
	Login(ctx context.Context, input model.LoginInput) (model.TokenPair, error)
	Refresh(ctx context.Context, input model.RefreshInput) (model.TokenPair, error)
	SwitchTenant(ctx context.Context, input model.SwitchTenantInput) (model.TokenPair, error)
	Logout(ctx context.Context, input model.LogoutInput) error
	Me(ctx context.Context, userID, tenantID string) (model.AuthUser, error)
	Sessions(ctx context.Context, userID, tenantID string) ([]model.RefreshSession, error)
	RevokeSession(ctx context.Context, userID, tenantID, sessionID string) error
	Roles(ctx context.Context, tenantID string) ([]model.RoleView, error)
	Permissions(ctx context.Context) ([]model.PermissionView, error)
	SetRolePermissions(ctx context.Context, tenantID, roleID string, permissions []string) error
	Users(ctx context.Context, tenantID string) ([]model.UserSummary, error)
	SetUserRoles(ctx context.Context, tenantID, userID string, roles []string) error
	Tenants(ctx context.Context, userID string) ([]model.TenantSummary, error)
	CreateTenant(ctx context.Context, ownerUserID string, input model.CreateTenantInput) (model.TenantSummary, error)
	ParseAccessToken(token string) (*AccessClaims, error)
	Menu(user model.AuthUser) []model.MenuItem
}

type authService struct {
	repository repository.AuthRepository
	config     config.AuthConfig
}

func NewAuthService(repository repository.AuthRepository, cfg config.AuthConfig) AuthService {
	return &authService{repository: repository, config: cfg}
}

func (s *authService) Register(ctx context.Context, input model.RegisterInput) (model.AuthUser, error) {
	input.Username, input.Email = strings.TrimSpace(input.Username), strings.ToLower(strings.TrimSpace(input.Email))
	if err := validateIdentity(input.Username, input.Email, input.Password); err != nil {
		return model.AuthUser{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return model.AuthUser{}, fmt.Errorf("hash password: %w", err)
	}
	return s.repository.CreateUser(ctx, input.Username, input.Email, string(hash))
}

func (s *authService) Login(ctx context.Context, input model.LoginInput) (model.TokenPair, error) {
	user, err := s.repository.FindCredentials(ctx, strings.ToLower(strings.TrimSpace(input.Email)))
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$C6UzMDM.H6dfI/f/IKcEe.ajGqL5vjvGp8f7iCSZ2mVf6.YWQ4Qtu"), []byte(input.Password))
		if errors.Is(err, repository.ErrUserNotFound) {
			return model.TokenPair{}, ErrInvalidCredentials
		}
		return model.TokenPair{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return model.TokenPair{}, ErrInvalidCredentials
	}
	if user.Status != "active" {
		return model.TokenPair{}, ErrAccountDisabled
	}
	tenant, err := s.repository.ResolveTenant(ctx, user.ID, input.TenantID)
	if err != nil {
		return model.TokenPair{}, mapTenantError(err)
	}
	authUser, err := s.repository.AuthUser(ctx, user.ID, tenant.ID)
	if err != nil {
		return model.TokenPair{}, mapTenantError(err)
	}
	return s.issueTokenPair(ctx, authUser, input.UserAgent, input.IPAddress)
}

func (s *authService) Refresh(ctx context.Context, input model.RefreshInput) (model.TokenPair, error) {
	return s.rotate(ctx, input.RefreshToken, "", input.UserAgent, input.IPAddress)
}
func (s *authService) SwitchTenant(ctx context.Context, input model.SwitchTenantInput) (model.TokenPair, error) {
	if input.TenantID == "" {
		return model.TokenPair{}, fmt.Errorf("%w: tenant_id is required", ErrInvalidAuthInput)
	}
	return s.rotate(ctx, input.RefreshToken, input.TenantID, input.UserAgent, input.IPAddress)
}

func (s *authService) rotate(ctx context.Context, refreshToken, targetTenantID, userAgent, ipAddress string) (model.TokenPair, error) {
	if len(refreshToken) < 32 {
		return model.TokenPair{}, ErrInvalidToken
	}
	raw, hash, err := newRefreshToken()
	if err != nil {
		return model.TokenPair{}, err
	}
	now := time.Now().UTC()
	next := model.RefreshSession{ID: newServiceUUID(), TokenHash: hash, UserAgent: truncate(userAgent, 512), IPAddress: truncate(ipAddress, 64), CreatedAt: now, ExpiresAt: now.Add(s.config.RefreshTokenTTL)}
	userID, tenantID, err := s.repository.RotateSession(ctx, hashToken(refreshToken), targetTenantID, next)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return model.TokenPair{}, ErrInvalidToken
		}
		return model.TokenPair{}, mapTenantError(err)
	}
	user, err := s.repository.AuthUser(ctx, userID, tenantID)
	if err != nil {
		return model.TokenPair{}, mapTenantError(err)
	}
	access, expiresAt, err := s.signAccessToken(user)
	if err != nil {
		return model.TokenPair{}, err
	}
	return model.TokenPair{AccessToken: access, RefreshToken: raw, TokenType: "Bearer", ExpiresAt: expiresAt, User: user}, nil
}

func (s *authService) Logout(ctx context.Context, input model.LogoutInput) error {
	if input.RefreshToken == "" {
		return ErrInvalidToken
	}
	return s.repository.RevokeSession(ctx, hashToken(input.RefreshToken))
}
func (s *authService) Me(ctx context.Context, userID, tenantID string) (model.AuthUser, error) {
	user, err := s.repository.AuthUser(ctx, userID, tenantID)
	return user, mapTenantError(err)
}
func (s *authService) Sessions(ctx context.Context, userID, tenantID string) ([]model.RefreshSession, error) {
	return s.repository.ListSessions(ctx, userID, tenantID)
}
func (s *authService) RevokeSession(ctx context.Context, userID, tenantID, sessionID string) error {
	return s.repository.RevokeSessionByID(ctx, userID, tenantID, sessionID)
}
func (s *authService) Roles(ctx context.Context, tenantID string) ([]model.RoleView, error) {
	return s.repository.ListRoles(ctx, tenantID)
}
func (s *authService) Permissions(ctx context.Context) ([]model.PermissionView, error) {
	return s.repository.ListPermissions(ctx)
}
func (s *authService) SetRolePermissions(ctx context.Context, tenantID, roleID string, permissions []string) error {
	return s.repository.SetRolePermissions(ctx, tenantID, roleID, permissions)
}
func (s *authService) Users(ctx context.Context, tenantID string) ([]model.UserSummary, error) {
	return s.repository.ListUsers(ctx, tenantID)
}
func (s *authService) SetUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return s.repository.SetUserRoles(ctx, tenantID, userID, roles)
}
func (s *authService) Tenants(ctx context.Context, userID string) ([]model.TenantSummary, error) {
	return s.repository.ListTenants(ctx, userID)
}
func (s *authService) CreateTenant(ctx context.Context, ownerUserID string, input model.CreateTenantInput) (model.TenantSummary, error) {
	input.Name, input.Slug = strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Slug))
	if len(input.Name) < 2 || len(input.Name) > 180 {
		return model.TenantSummary{}, fmt.Errorf("%w: tenant name must be 2-180 characters", ErrInvalidAuthInput)
	}
	if !tenantSlugPattern.MatchString(input.Slug) {
		return model.TenantSummary{}, fmt.Errorf("%w: tenant slug must be 3-80 lowercase letters, numbers, or hyphens", ErrInvalidAuthInput)
	}
	return s.repository.CreateTenant(ctx, ownerUserID, input)
}

func (s *authService) ParseAccessToken(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithIssuer(s.config.Issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid || claims.Subject == "" || claims.TenantID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *authService) Menu(user model.AuthUser) []model.MenuItem {
	definitions := []model.MenuItem{{ID: "dashboard", Label: "总览", Href: "/admin", Icon: "layout-dashboard", Permission: "dashboard.view"}, {ID: "entities", Label: "数据模型", Href: "/admin/entities", Icon: "database-zap", Permission: "entity.view"}, {ID: "records", Label: "业务数据", Href: "/admin/data", Icon: "table-properties", Permission: "record.view"}, {ID: "forms", Label: "表单构建器", Href: "/admin/forms", Icon: "panels-top-left", Permission: "form.view"}, {ID: "workflows", Label: "工作流", Href: "/admin/workflows", Icon: "workflow", Permission: "workflow.view"}, {ID: "files", Label: "文件空间", Href: "/admin/files", Icon: "folder-open", Permission: "file.view"}, {ID: "ai", Label: "AI 助手", Href: "/admin/ai", Icon: "bot", Permission: "ai.chat"}, {ID: "billing", Label: "套餐与用量", Href: "/admin/billing", Icon: "credit-card", Permission: "billing.manage"}, {ID: "users", Label: "用户管理", Href: "/admin/users", Icon: "users", Permission: "user.view"}, {ID: "roles", Label: "角色权限", Href: "/admin/access", Icon: "shield-check", Permission: "role.manage"}, {ID: "settings", Label: "系统设置", Href: "/admin/settings", Icon: "settings", Permission: "system.manage"}}
	allowed := make(map[string]struct{}, len(user.Permissions))
	for _, permission := range user.Permissions {
		allowed[permission] = struct{}{}
	}
	menu := make([]model.MenuItem, 0, len(definitions))
	for _, item := range definitions {
		if _, ok := allowed[item.Permission]; ok {
			menu = append(menu, item)
		}
	}
	return menu
}

func (s *authService) issueTokenPair(ctx context.Context, user model.AuthUser, userAgent, ipAddress string) (model.TokenPair, error) {
	raw, hash, err := newRefreshToken()
	if err != nil {
		return model.TokenPair{}, err
	}
	now := time.Now().UTC()
	session := model.RefreshSession{ID: newServiceUUID(), TenantID: user.ActiveTenantID, UserID: user.ID, TokenHash: hash, UserAgent: truncate(userAgent, 512), IPAddress: truncate(ipAddress, 64), CreatedAt: now, ExpiresAt: now.Add(s.config.RefreshTokenTTL)}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return model.TokenPair{}, err
	}
	access, expiresAt, err := s.signAccessToken(user)
	if err != nil {
		_ = s.repository.RevokeSession(ctx, hash)
		return model.TokenPair{}, err
	}
	return model.TokenPair{AccessToken: access, RefreshToken: raw, TokenType: "Bearer", ExpiresAt: expiresAt, User: user}, nil
}

func (s *authService) signAccessToken(user model.AuthUser) (string, time.Time, error) {
	now, expiresAt := time.Now().UTC(), time.Now().UTC().Add(s.config.AccessTokenTTL)
	claims := AccessClaims{Email: user.Email, TenantID: user.ActiveTenantID, Roles: user.Roles, Permissions: user.Permissions, RegisteredClaims: jwt.RegisteredClaims{Issuer: s.config.Issuer, Subject: user.ID, ID: newServiceUUID(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt)}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.JWTSecret))
	return raw, expiresAt, err
}

func validateIdentity(username, email, password string) error {
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("%w: username must be 3-64 characters", ErrInvalidAuthInput)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: email is invalid", ErrInvalidAuthInput)
	}
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("%w: password must be 12-72 characters", ErrInvalidAuthInput)
	}
	return nil
}
func mapTenantError(err error) error {
	if errors.Is(err, repository.ErrTenantAccess) {
		return ErrTenantAccess
	}
	return err
}
func newRefreshToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(buffer)
	return raw, hashToken(raw), nil
}
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func newServiceUUID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return ""
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	value := hex.EncodeToString(buffer)
	return value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
}
func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
