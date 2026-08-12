package model

import (
	"encoding/json"
	"time"
)

type Workflow struct {
	ID          string          `gorm:"type:uuid;primaryKey"`
	TenantID    string          `gorm:"type:uuid;not null;uniqueIndex:idx_workflow_tenant_slug"`
	EntityID    string          `gorm:"type:uuid;not null;index"`
	Name        string          `gorm:"size:120;not null"`
	Slug        string          `gorm:"size:80;not null;uniqueIndex:idx_workflow_tenant_slug"`
	Description string          `gorm:"size:500;not null"`
	Nodes       json.RawMessage `gorm:"type:jsonb;not null"`
	Edges       json.RawMessage `gorm:"type:jsonb;not null"`
	Version     int             `gorm:"not null"`
	Status      string          `gorm:"size:24;not null;index"`
	CreatedAt   time.Time       `gorm:"not null"`
	UpdatedAt   time.Time       `gorm:"not null"`
}

func (Workflow) TableName() string { return "workflows" }

type WorkflowNode struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
	X      int            `json:"x"`
	Y      int            `json:"y"`
}

type WorkflowEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
}

type WorkflowDefinition struct {
	ID          string         `json:"id"`
	EntityID    string         `json:"entity_id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Description string         `json:"description"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	Version     int            `json:"version"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type DefineWorkflowInput struct {
	ID              string         `json:"id,omitempty"`
	EntityID        string         `json:"entity_id"`
	Name            string         `json:"name"`
	Slug            string         `json:"slug"`
	Description     string         `json:"description"`
	ExpectedVersion int            `json:"expected_version,omitempty"`
	Nodes           []WorkflowNode `json:"nodes"`
	Edges           []WorkflowEdge `json:"edges"`
}

type WorkflowInstance struct {
	ID            string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID      string    `gorm:"type:uuid;not null;index:idx_instance_tenant_status" json:"-"`
	WorkflowID    string    `gorm:"type:uuid;not null;index" json:"workflow_id"`
	EntityID      string    `gorm:"type:uuid;not null;index" json:"entity_id"`
	RecordID      string    `gorm:"type:uuid;not null;index" json:"record_id"`
	CurrentNodeID string    `gorm:"size:80;not null" json:"current_node_id"`
	Status        string    `gorm:"size:24;not null;index:idx_instance_tenant_status" json:"status"`
	Version       int       `gorm:"not null" json:"version"`
	SubmittedBy   string    `gorm:"type:uuid;not null" json:"submitted_by"`
	CreatedAt     time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt     time.Time `gorm:"not null" json:"updated_at"`
}

func (WorkflowInstance) TableName() string { return "workflow_instances" }

type WorkflowAction struct {
	ID         string    `gorm:"type:uuid;primaryKey"`
	TenantID   string    `gorm:"type:uuid;not null;index"`
	InstanceID string    `gorm:"type:uuid;not null;index"`
	NodeID     string    `gorm:"size:80;not null"`
	ActorID    string    `gorm:"type:uuid;not null"`
	Action     string    `gorm:"size:24;not null"`
	Comment    string    `gorm:"size:1000;not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (WorkflowAction) TableName() string { return "workflow_actions" }

type Notification struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID   string     `gorm:"type:uuid;not null;index:idx_notification_tenant_user" json:"-"`
	UserID     string     `gorm:"type:uuid;not null;index:idx_notification_tenant_user" json:"user_id"`
	InstanceID string     `gorm:"type:uuid;not null;index" json:"instance_id"`
	Channel    string     `gorm:"size:24;not null;index" json:"channel"`
	Recipient  string     `gorm:"size:500;not null" json:"recipient"`
	Subject    string     `gorm:"size:200;not null" json:"subject"`
	Body       string     `gorm:"size:4000;not null" json:"body"`
	Status     string     `gorm:"size:24;not null;index" json:"status"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `gorm:"not null" json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

type StartWorkflowInput struct {
	RecordID string `json:"record_id"`
}
type WorkflowActionInput struct {
	Action          string `json:"action"`
	Comment         string `json:"comment"`
	ExpectedVersion int    `json:"expected_version"`
}

type WorkflowTransition struct {
	CurrentNodeID string
	Status        string
	Notifications []Notification
}
