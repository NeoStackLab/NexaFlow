package model

import (
	"encoding/json"
	"time"
)

type AuditEvent struct {
	ID         string          `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID   string          `gorm:"type:uuid;not null;index:idx_audit_tenant_created" json:"-"`
	ActorID    string          `gorm:"type:uuid;not null;index" json:"actor_id"`
	Action     string          `gorm:"size:80;not null;index" json:"action"`
	Resource   string          `gorm:"size:160;not null;index" json:"resource"`
	ResourceID string          `gorm:"size:160;not null;default:''" json:"resource_id,omitempty"`
	Method     string          `gorm:"size:12;not null" json:"method"`
	Path       string          `gorm:"size:500;not null" json:"path"`
	StatusCode int             `gorm:"not null" json:"status_code"`
	RequestID  string          `gorm:"size:80;not null;index" json:"request_id"`
	IPAddress  string          `gorm:"size:64;not null" json:"ip_address"`
	Metadata   json.RawMessage `gorm:"type:jsonb;not null" json:"metadata"`
	CreatedAt  time.Time       `gorm:"not null;index:idx_audit_tenant_created,sort:desc" json:"created_at"`
}

func (AuditEvent) TableName() string { return "audit_events" }
