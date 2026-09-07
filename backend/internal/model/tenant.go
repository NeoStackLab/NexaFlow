package model

import (
	"encoding/json"
	"strings"
	"time"
)

type TenantSettings struct {
	TenantID     string          `gorm:"type:uuid;primaryKey" json:"tenant_id"`
	Timezone     string          `gorm:"size:64;not null" json:"timezone"`
	Locale       string          `gorm:"size:16;not null" json:"locale"`
	Branding     json.RawMessage `gorm:"type:jsonb;not null" json:"branding"`
	CustomDomain *string         `gorm:"size:253;uniqueIndex" json:"custom_domain,omitempty"`
	Security     json.RawMessage `gorm:"type:jsonb;not null" json:"security"`
	Version      int             `gorm:"not null;default:1" json:"version"`
	CreatedAt    time.Time       `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time       `gorm:"not null" json:"updated_at"`
}

func (TenantSettings) TableName() string { return "tenant_settings" }

func NewTenantSettings(tenantID, timezone, locale string, now time.Time) TenantSettings {
	if strings.TrimSpace(timezone) == "" {
		timezone = "UTC"
	}
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}
	return TenantSettings{
		TenantID: tenantID, Timezone: timezone, Locale: locale,
		Branding: []byte(`{}`),
		Security: []byte(`{"require_mfa":false,"minimum_password_length":12,"session_ttl_minutes":1440,"allowed_email_domains":[],"ip_allowlist":[]}`),
		Version:  1, CreatedAt: now, UpdatedAt: now,
	}
}

type TenantBranding struct {
	LogoURL      string `json:"logo_url,omitempty"`
	PrimaryColor string `json:"primary_color,omitempty"`
}

type TenantSecuritySettings struct {
	RequireMFA            bool     `json:"require_mfa"`
	MinimumPasswordLength int      `json:"minimum_password_length"`
	SessionTTLMinutes     int      `json:"session_ttl_minutes"`
	AllowedEmailDomains   []string `json:"allowed_email_domains"`
	IPAllowlist           []string `json:"ip_allowlist"`
}

type TenantSettingsView struct {
	Timezone     string                 `json:"timezone"`
	Locale       string                 `json:"locale"`
	Branding     TenantBranding         `json:"branding"`
	CustomDomain string                 `json:"custom_domain,omitempty"`
	Security     TenantSecuritySettings `json:"security"`
	Version      int                    `json:"version"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type TenantView struct {
	ID        string             `json:"id"`
	Slug      string             `json:"slug"`
	Name      string             `json:"name"`
	Status    string             `json:"status"`
	Version   int                `json:"version"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	Settings  TenantSettingsView `json:"settings"`
}

type UpdateTenantInput struct {
	Name            string `json:"name"`
	ExpectedVersion int    `json:"expected_version"`
}

type UpdateTenantSettingsInput struct {
	Timezone        string                 `json:"timezone"`
	Locale          string                 `json:"locale"`
	Branding        TenantBranding         `json:"branding"`
	CustomDomain    string                 `json:"custom_domain"`
	Security        TenantSecuritySettings `json:"security"`
	ExpectedVersion int                    `json:"expected_version"`
}
