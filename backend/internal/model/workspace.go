package model

import (
	"encoding/json"
	"time"
)

type FileAsset struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID    string    `gorm:"type:uuid;not null;index:idx_file_tenant_created,priority:1" json:"-"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	ContentType string    `gorm:"size:160;not null" json:"content_type"`
	Size        int64     `gorm:"not null" json:"size"`
	StorageKey  string    `gorm:"size:700;not null;uniqueIndex" json:"-"`
	Provider    string    `gorm:"size:24;not null" json:"provider"`
	Status      string    `gorm:"size:24;not null;index" json:"status"`
	CreatedBy   string    `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt   time.Time `gorm:"not null;index:idx_file_tenant_created,priority:2,sort:desc" json:"created_at"`
}

func (FileAsset) TableName() string { return "file_assets" }

type Dashboard struct {
	ID        string          `gorm:"type:uuid;primaryKey"`
	TenantID  string          `gorm:"type:uuid;not null;uniqueIndex"`
	Widgets   json.RawMessage `gorm:"type:jsonb;not null"`
	Version   int             `gorm:"not null"`
	UpdatedBy string          `gorm:"type:uuid;not null"`
	CreatedAt time.Time       `gorm:"not null"`
	UpdatedAt time.Time       `gorm:"not null"`
}

func (Dashboard) TableName() string { return "dashboards" }

type DashboardWidget struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	EntityID string `json:"entity_id,omitempty"`
	Field    string `json:"field,omitempty"`
	Width    int    `json:"width"`
}

type DashboardView struct {
	Widgets []DashboardWidget `json:"widgets"`
	Values  map[string]float64 `json:"values"`
	Version int                `json:"version"`
}

type SaveDashboardInput struct {
	Widgets         []DashboardWidget `json:"widgets"`
	ExpectedVersion int               `json:"expected_version"`
}
