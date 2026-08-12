package model

import (
	"encoding/json"
	"time"
)

type Entity struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	TenantID    string    `gorm:"type:uuid;not null;uniqueIndex:idx_entity_tenant_slug"`
	Name        string    `gorm:"size:120;not null"`
	Slug        string    `gorm:"size:80;not null;uniqueIndex:idx_entity_tenant_slug"`
	Description string    `gorm:"size:500;not null"`
	Version     int       `gorm:"not null"`
	Status      string    `gorm:"size:24;not null;index"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (Entity) TableName() string { return "entities" }

type EntityField struct {
	ID           string          `gorm:"type:uuid;primaryKey"`
	TenantID     string          `gorm:"type:uuid;not null;index"`
	EntityID     string          `gorm:"type:uuid;not null;uniqueIndex:idx_entity_field_name"`
	Name         string          `gorm:"size:80;not null;uniqueIndex:idx_entity_field_name"`
	Label        string          `gorm:"size:120;not null"`
	Type         string          `gorm:"size:32;not null;index"`
	Required     bool            `gorm:"not null"`
	DefaultValue json.RawMessage `gorm:"type:jsonb"`
	Options      json.RawMessage `gorm:"type:jsonb"`
	Position     int             `gorm:"not null"`
	CreatedAt    time.Time       `gorm:"not null"`
	UpdatedAt    time.Time       `gorm:"not null"`
}

func (EntityField) TableName() string { return "entity_fields" }

type FieldDefinition struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Options  []string `json:"options,omitempty"`
	Position int      `json:"position"`
}

type EntityDefinition struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
	Version     int               `json:"version"`
	Status      string            `json:"status"`
	Fields      []FieldDefinition `json:"fields"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type DefineEntityInput struct {
	ID              string            `json:"id,omitempty"`
	Name            string            `json:"name"`
	Slug            string            `json:"slug"`
	Description     string            `json:"description"`
	ExpectedVersion int               `json:"expected_version,omitempty"`
	Fields          []FieldDefinition `json:"fields"`
}
