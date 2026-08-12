package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authRepositoryStub struct {
	credentials   model.BootstrapUser
	user          model.AuthUser
	createUser    model.AuthUser
	session       model.RefreshSession
	rotated       model.RefreshSession
	rotateUser    string
	rotateTenant  string
	resolveTenant model.TenantSummary
	rotateError   error
	revokedHash   string
}

func (s *authRepositoryStub) CreateUser(_ context.Context, username, email, passwordHash string) (model.AuthUser, error) {
	s.credentials = model.BootstrapUser{Username: username, Email: email, PasswordHash: passwordHash}
	return s.createUser, nil
}
func (s *authRepositoryStub) FindCredentials(context.Context, string) (model.BootstrapUser, error) {
	if s.credentials.ID == "" {
		return model.BootstrapUser{}, repository.ErrUserNotFound
	}
	return s.credentials, nil
}
func (s *authRepositoryStub) ResolveTenant(context.Context, string, string) (model.TenantSummary, error) {
	if s.resolveTenant.ID == "" {
		return model.TenantSummary{}, repository.ErrTenantAccess
	}
	return s.resolveTenant, nil
}
func (s *authRepositoryStub) AuthUser(context.Context, string, string) (model.AuthUser, error) {
	return s.user, nil
}
func (s *authRepositoryStub) CreateSession(_ context.Context, session model.RefreshSession) error {
	s.session = session
	return nil
}
func (s *authRepositoryStub) RotateSession(_ context.Context, _, _ string, next model.RefreshSession) (string, string, error) {
	if s.rotateError != nil {
		return "", "", s.rotateError
	}
	if s.rotateUser == "" {
		return "", "", repository.ErrSessionNotFound
	}
	s.rotated = next
	return s.rotateUser, s.rotateTenant, nil
}
func (s *authRepositoryStub) RevokeSession(_ context.Context, hash string) error {
	s.revokedHash = hash
	return nil
}
func (s *authRepositoryStub) ListSessions(context.Context, string, string) ([]model.RefreshSession, error) {
	return nil, nil
}
func (s *authRepositoryStub) RevokeSessionByID(context.Context, string, string, string) error {
	return nil
}
func (s *authRepositoryStub) ListRoles(context.Context, string) ([]model.RoleView, error) {
	return nil, nil
}
func (s *authRepositoryStub) ListPermissions(context.Context) ([]model.PermissionView, error) {
	return nil, nil
}
func (s *authRepositoryStub) SetRolePermissions(context.Context, string, string, []string) error {
	return nil
}
func (s *authRepositoryStub) ListUsers(context.Context, string) ([]model.UserSummary, error) {
	return nil, nil
}
func (s *authRepositoryStub) SetUserRoles(context.Context, string, string, []string) error {
	return nil
}
func (s *authRepositoryStub) ListTenants(context.Context, string) ([]model.TenantSummary, error) {
	return nil, nil
}
func (s *authRepositoryStub) CreateTenant(context.Context, string, model.CreateTenantInput) (model.TenantSummary, error) {
	return model.TenantSummary{}, nil
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{JWTSecret: "a-test-secret-that-is-at-least-32-characters", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 24 * time.Hour, Issuer: "nexaflow-test"}
}

func TestAuthServiceRegisterHashesPassword(t *testing.T) {
	repo := &authRepositoryStub{createUser: model.AuthUser{ID: "user-1"}}
	service := NewAuthService(repo, testAuthConfig())
	_, err := service.Register(context.Background(), model.RegisterInput{Username: "staff", Email: "staff@example.com", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if repo.credentials.PasswordHash == "correct-horse-battery" {
		t.Fatal("password was stored in plaintext")
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.credentials.PasswordHash), []byte("correct-horse-battery")) != nil {
		t.Fatal("stored password hash does not verify")
	}
}

func TestAuthServiceLoginIssuesVerifiableTokens(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-horse-battery"), bcrypt.MinCost)
	repo := &authRepositoryStub{credentials: model.BootstrapUser{ID: "user-1", Email: "admin@example.com", PasswordHash: string(hash), Status: "active"}, resolveTenant: model.TenantSummary{ID: "tenant-a", Name: "Tenant A"}, user: model.AuthUser{ID: "user-1", Email: "admin@example.com", ActiveTenantID: "tenant-a", Roles: []string{"super_admin"}, Permissions: []string{"dashboard.view"}}}
	service := NewAuthService(repo, testAuthConfig())
	pair, err := service.Login(context.Background(), model.LoginInput{Email: "admin@example.com", Password: "correct-horse-battery", UserAgent: "test", IPAddress: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("token pair is incomplete")
	}
	claims, err := service.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.Subject != "user-1" || len(claims.Permissions) != 1 {
		t.Fatalf("claims = %#v", claims)
	}
	if claims.TenantID != "tenant-a" || repo.session.TenantID != "tenant-a" {
		t.Fatalf("tenant claim/session = %q/%q", claims.TenantID, repo.session.TenantID)
	}
	if repo.session.TokenHash == pair.RefreshToken {
		t.Fatal("refresh token was stored in plaintext")
	}
}

func TestAuthServiceRejectsUnknownLogin(t *testing.T) {
	service := NewAuthService(&authRepositoryStub{}, testAuthConfig())
	_, err := service.Login(context.Background(), model.LoginInput{Email: "missing@example.com", Password: "wrong"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthServiceRotatesRefreshToken(t *testing.T) {
	repo := &authRepositoryStub{rotateUser: "user-1", rotateTenant: "tenant-a", user: model.AuthUser{ID: "user-1", Email: "admin@example.com", ActiveTenantID: "tenant-a"}}
	service := NewAuthService(repo, testAuthConfig())
	pair, err := service.Refresh(context.Background(), model.RefreshInput{RefreshToken: "abcdefghijklmnopqrstuvwxyz-0123456789-token"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if pair.RefreshToken == "" || repo.rotated.TokenHash == pair.RefreshToken {
		t.Fatal("refresh rotation did not produce a hashed replacement")
	}
	claims, err := service.ParseAccessToken(pair.AccessToken)
	if err != nil || claims.TenantID != "tenant-a" {
		t.Fatalf("refresh tenant claims = %#v, err = %v", claims, err)
	}
}

func TestAuthServiceRejectsTenantSwitchWithoutMembership(t *testing.T) {
	repo := &authRepositoryStub{rotateError: repository.ErrTenantAccess}
	service := NewAuthService(repo, testAuthConfig())
	_, err := service.SwitchTenant(context.Background(), model.SwitchTenantInput{TenantID: "tenant-b", RefreshToken: "abcdefghijklmnopqrstuvwxyz-0123456789-token"})
	if !errors.Is(err, ErrTenantAccess) {
		t.Fatalf("SwitchTenant() error = %v, want ErrTenantAccess", err)
	}
}

func TestAuthServiceSwitchesTenantInBothTokens(t *testing.T) {
	repo := &authRepositoryStub{rotateUser: "user-1", rotateTenant: "tenant-b", user: model.AuthUser{ID: "user-1", Email: "admin@example.com", ActiveTenantID: "tenant-b"}}
	service := NewAuthService(repo, testAuthConfig())
	pair, err := service.SwitchTenant(context.Background(), model.SwitchTenantInput{TenantID: "tenant-b", RefreshToken: "abcdefghijklmnopqrstuvwxyz-0123456789-token"})
	if err != nil {
		t.Fatalf("SwitchTenant() error = %v", err)
	}
	claims, err := service.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.TenantID != "tenant-b" {
		t.Fatalf("tenant claim = %q, want tenant-b", claims.TenantID)
	}
}

func TestAuthServiceRejectsAccessTokenWithoutTenant(t *testing.T) {
	service := NewAuthService(&authRepositoryStub{}, testAuthConfig())
	now := time.Now().UTC()
	claims := AccessClaims{RegisteredClaims: jwt.RegisteredClaims{Issuer: "nexaflow-test", Subject: "user-1", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testAuthConfig().JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ParseAccessToken(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseAccessToken() error = %v, want ErrInvalidToken", err)
	}
}

func TestAuthServiceFiltersMenuByPermission(t *testing.T) {
	service := NewAuthService(&authRepositoryStub{}, testAuthConfig())
	menu := service.Menu(model.AuthUser{Permissions: []string{"dashboard.view", "role.manage"}})
	if len(menu) != 2 || menu[0].ID != "dashboard" || menu[1].ID != "roles" {
		t.Fatalf("Menu() = %#v", menu)
	}
}
