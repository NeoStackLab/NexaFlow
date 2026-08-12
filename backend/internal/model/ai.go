package model

import (
	"encoding/json"
	"time"
)

type KnowledgeDocument struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID    string    `gorm:"type:uuid;not null;index:idx_knowledge_tenant_status" json:"-"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	ContentType string    `gorm:"size:120;not null" json:"content_type"`
	Size        int64     `gorm:"not null" json:"size"`
	ChunkCount  int       `gorm:"not null" json:"chunk_count"`
	Status      string    `gorm:"size:24;not null;index:idx_knowledge_tenant_status" json:"status"`
	CreatedBy   string    `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}

func (KnowledgeDocument) TableName() string { return "knowledge_documents" }

type KnowledgeChunk struct {
	ID         string    `gorm:"type:uuid;primaryKey"`
	TenantID   string    `gorm:"type:uuid;not null;index:idx_chunk_tenant_document"`
	DocumentID string    `gorm:"type:uuid;not null;index:idx_chunk_tenant_document"`
	Position   int       `gorm:"not null"`
	Content    string    `gorm:"type:text;not null"`
	Embedding  string    `gorm:"type:vector(1536);not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (KnowledgeChunk) TableName() string { return "knowledge_chunks" }

type KnowledgeChunkInput struct {
	Position  int
	Content   string
	Embedding []float32
}
type KnowledgeSource struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	ChunkID      string  `json:"chunk_id"`
	Content      string  `json:"content"`
	Score        float64 `json:"score"`
}

type AIConversation struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID  string    `gorm:"type:uuid;not null;index" json:"-"`
	UserID    string    `gorm:"type:uuid;not null;index" json:"user_id"`
	Title     string    `gorm:"size:200;not null" json:"title"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (AIConversation) TableName() string { return "ai_conversations" }

type AIMessage struct {
	ID             string          `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID       string          `gorm:"type:uuid;not null;index" json:"-"`
	ConversationID string          `gorm:"type:uuid;not null;index" json:"conversation_id"`
	Role           string          `gorm:"size:24;not null" json:"role"`
	Content        string          `gorm:"type:text;not null" json:"content"`
	Sources        json.RawMessage `gorm:"type:jsonb;not null" json:"sources"`
	ToolCalls      json.RawMessage `gorm:"type:jsonb;not null" json:"tool_calls"`
	InputTokens    int             `gorm:"not null" json:"input_tokens"`
	OutputTokens   int             `gorm:"not null" json:"output_tokens"`
	CreatedAt      time.Time       `gorm:"not null" json:"created_at"`
}

func (AIMessage) TableName() string { return "ai_messages" }

type AIAskInput struct {
	ConversationID string `json:"conversation_id,omitempty"`
	Message        string `json:"message"`
}
type AIAnswer struct {
	ConversationID string            `json:"conversation_id"`
	Message        AIMessage         `json:"message"`
	Sources        []KnowledgeSource `json:"sources"`
}
