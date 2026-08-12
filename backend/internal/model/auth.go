package model

import "time"

type RegisterInput struct{ Username, Email, Password, TenantID string }
type LoginInput struct{ Email, Password, UserAgent, IPAddress, TenantID string }
type RefreshInput struct{ RefreshToken, UserAgent, IPAddress string }
type LogoutInput struct{ RefreshToken string }

type AuthUser struct {
	ID             string          `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Status         string          `json:"status"`
	Roles          []string        `json:"roles"`
	Permissions    []string        `json:"permissions"`
	ActiveTenantID string          `json:"active_tenant_id"`
	Tenants        []TenantSummary `json:"tenants"`
}

type TenantSummary struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         AuthUser  `json:"user"`
}

type MenuItem struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Href       string `json:"href"`
	Icon       string `json:"icon"`
	Permission string `json:"permission,omitempty"`
}

type RoleView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
}

type PermissionView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type UserSummary struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Status   string   `json:"status"`
	Roles    []string `json:"roles"`
}

type CreateTenantInput struct{ Name, Slug string }
type SwitchTenantInput struct{ TenantID, RefreshToken, UserAgent, IPAddress string }
