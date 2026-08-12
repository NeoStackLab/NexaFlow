package model

import "time"

type InstallStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Mode      string `json:"mode"`
	LockPath  string `json:"lock_path,omitempty"`
}

type EnvironmentCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Version     string `json:"version,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
	Required    bool   `json:"required"`
}

type CapabilityCheck struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Message    string `json:"message"`
}

type InstallReadiness struct {
	Infrastructure []EnvironmentCheck `json:"infrastructure"`
	Capabilities   []CapabilityCheck  `json:"capabilities"`
}

type DatabaseInput struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"sslmode"`
}

type RedisInput struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Database int    `json:"database"`
}

type AdminInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CompanyInput struct {
	Name            string `json:"name"`
	Industry        string `json:"industry"`
	DefaultLanguage string `json:"default_language"`
	Timezone        string `json:"timezone"`
}

type CompleteInstallationInput struct {
	Admin   AdminInput   `json:"admin"`
	Company CompanyInput `json:"company"`
}

type InstallRuntimeConfig struct {
	Database DatabaseInput `json:"database"`
	Redis    RedisInput    `json:"redis"`
	Written  time.Time     `json:"written_at"`
}

type InstallationResult struct {
	AdminURL string `json:"admin_url"`
	Username string `json:"username"`
	LockPath string `json:"lock_path"`
}

type BootstrapUser struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"size:64;not null;uniqueIndex"`
	Email        string    `gorm:"size:320;not null;uniqueIndex"`
	PasswordHash string    `gorm:"size:255;not null"`
	Status       string    `gorm:"size:32;not null;index"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (BootstrapUser) TableName() string { return "users" }

type BootstrapRole struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"size:64;not null;uniqueIndex"`
	DisplayName string    `gorm:"size:128;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (BootstrapRole) TableName() string { return "roles" }

type BootstrapPermission struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"size:128;not null;uniqueIndex"`
	Description string    `gorm:"size:255;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (BootstrapPermission) TableName() string { return "permissions" }

type BootstrapRolePermission struct {
	RoleID       string    `gorm:"type:uuid;primaryKey"`
	PermissionID string    `gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (BootstrapRolePermission) TableName() string { return "role_permissions" }

type TenantRolePermission struct {
	TenantID     string    `gorm:"type:uuid;primaryKey"`
	RoleID       string    `gorm:"type:uuid;primaryKey"`
	PermissionID string    `gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (TenantRolePermission) TableName() string { return "tenant_role_permissions" }

type BootstrapUserRole struct {
	UserID    string    `gorm:"type:uuid;primaryKey"`
	RoleID    string    `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"not null"`
}

func (BootstrapUserRole) TableName() string { return "user_roles" }

type BootstrapCompany struct {
	ID              string    `gorm:"type:uuid;primaryKey"`
	TenantID        *string   `gorm:"type:uuid;uniqueIndex"`
	Name            string    `gorm:"size:180;not null"`
	Industry        string    `gorm:"size:64;not null"`
	DefaultLanguage string    `gorm:"size:16;not null"`
	Timezone        string    `gorm:"size:64;not null"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

func (BootstrapCompany) TableName() string { return "companies" }

type BootstrapSetting struct {
	Key       string    `gorm:"size:128;primaryKey"`
	Value     string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (BootstrapSetting) TableName() string { return "system_settings" }

type RefreshSession struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID  string     `gorm:"type:uuid;not null;default:'00000000-0000-0000-0000-000000000000';index" json:"tenant_id"`
	UserID    string     `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	UserAgent string     `gorm:"size:512;not null" json:"user_agent"`
	IPAddress string     `gorm:"size:64;not null" json:"ip_address"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `gorm:"not null" json:"created_at"`
}

func (RefreshSession) TableName() string { return "refresh_sessions" }

type Tenant struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	Slug      string    `gorm:"size:80;not null;uniqueIndex" json:"slug"`
	Name      string    `gorm:"size:180;not null" json:"name"`
	Status    string    `gorm:"size:32;not null;index" json:"status"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (Tenant) TableName() string { return "tenants" }

type TenantMembership struct {
	TenantID  string    `gorm:"type:uuid;primaryKey"`
	UserID    string    `gorm:"type:uuid;primaryKey"`
	Status    string    `gorm:"size:32;not null;index"`
	CreatedAt time.Time `gorm:"not null"`
}

func (TenantMembership) TableName() string { return "tenant_memberships" }

type TenantUserRole struct {
	TenantID  string    `gorm:"type:uuid;primaryKey"`
	UserID    string    `gorm:"type:uuid;primaryKey"`
	RoleID    string    `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"not null"`
}

func (TenantUserRole) TableName() string { return "tenant_user_roles" }
