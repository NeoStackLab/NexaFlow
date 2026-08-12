package model

import (
	"encoding/json"
	"time"
)

type DynamicRecord struct {
	ID        string          `gorm:"type:uuid;primaryKey"`
	TenantID  string          `gorm:"type:uuid;not null;index:idx_record_tenant_entity_created,priority:1"`
	EntityID  string          `gorm:"type:uuid;not null;index:idx_record_tenant_entity_created,priority:2"`
	Values    json.RawMessage `gorm:"type:jsonb;not null"`
	Version   int             `gorm:"not null"`
	Status    string          `gorm:"size:24;not null;index"`
	CreatedBy string          `gorm:"type:uuid;not null"`
	UpdatedBy string          `gorm:"type:uuid;not null"`
	CreatedAt time.Time       `gorm:"not null;index:idx_record_tenant_entity_created,priority:3,sort:desc"`
	UpdatedAt time.Time       `gorm:"not null"`
}

func (DynamicRecord) TableName() string { return "dynamic_records" }

type RecordView struct {
	ID        string         `json:"id"`
	EntityID  string         `json:"entity_id"`
	Values    map[string]any `json:"values"`
	Version   int            `json:"version"`
	CreatedBy string         `json:"created_by"`
	UpdatedBy string         `json:"updated_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type WriteRecordInput struct {
	Values          map[string]any `json:"values"`
	ExpectedVersion int            `json:"expected_version,omitempty"`
}

type Form struct {
	ID          string          `gorm:"type:uuid;primaryKey"`
	TenantID    string          `gorm:"type:uuid;not null;uniqueIndex:idx_form_tenant_slug"`
	EntityID    string          `gorm:"type:uuid;not null;index"`
	Name        string          `gorm:"size:120;not null"`
	Slug        string          `gorm:"size:80;not null;uniqueIndex:idx_form_tenant_slug"`
	Description string          `gorm:"size:500;not null"`
	JSONSchema  json.RawMessage `gorm:"type:jsonb;not null"`
	Layout      json.RawMessage `gorm:"type:jsonb;not null"`
	Version     int             `gorm:"not null"`
	Status      string          `gorm:"size:24;not null;index"`
	CreatedAt   time.Time       `gorm:"not null"`
	UpdatedAt   time.Time       `gorm:"not null"`
}

func (Form) TableName() string { return "forms" }

type FormComponent struct {
	FieldName string         `json:"field_name"`
	Widget    string         `json:"widget"`
	Label     string         `json:"label"`
	Required  bool           `json:"required"`
	Position  int            `json:"position"`
	Props     map[string]any `json:"props,omitempty"`
}

type FormDefinition struct {
	ID          string          `json:"id"`
	EntityID    string          `json:"entity_id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	JSONSchema  map[string]any  `json:"json_schema"`
	Components  []FormComponent `json:"components"`
	Version     int             `json:"version"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type DefineFormInput struct {
	ID              string          `json:"id,omitempty"`
	EntityID        string          `json:"entity_id"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	ExpectedVersion int             `json:"expected_version,omitempty"`
	Components      []FormComponent `json:"components"`
}
