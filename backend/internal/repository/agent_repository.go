package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
)

var ErrConversationNotFound = errors.New("AI conversation not found")

type AgentRepository struct{ knowledge *KnowledgeRepository }

func NewAgentRepository(install InstallRepository) *AgentRepository {
	return &AgentRepository{knowledge: NewKnowledgeRepository(install)}
}
func (r *AgentRepository) SaveExchange(ctx context.Context, tenantID, userID string, input model.AIAskInput, answer string, sources []model.KnowledgeSource, toolCalls []map[string]any, inputTokens, outputTokens int) (model.AIAnswer, error) {
	var result model.AIAnswer
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			conversation := model.AIConversation{ID: input.ConversationID}
			if input.ConversationID == "" {
				title := []rune(input.Message)
				if len(title) > 80 {
					title = title[:80]
				}
				conversation = model.AIConversation{ID: newUUID(), TenantID: tenantID, UserID: userID, Title: string(title), CreatedAt: now, UpdatedAt: now}
				if err := tx.Create(&conversation).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Where("tenant_id = ? AND user_id = ? AND id = ?", tenantID, userID, input.ConversationID).First(&conversation).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return ErrConversationNotFound
					}
					return err
				}
				if err := tx.Model(&conversation).Update("updated_at", now).Error; err != nil {
					return err
				}
			}
			empty, _ := json.Marshal([]any{})
			userMessage := model.AIMessage{ID: newUUID(), TenantID: tenantID, ConversationID: conversation.ID, Role: "user", Content: input.Message, Sources: empty, ToolCalls: empty, CreatedAt: now}
			if err := tx.Create(&userMessage).Error; err != nil {
				return err
			}
			sourceJSON, _ := json.Marshal(sources)
			toolJSON, _ := json.Marshal(toolCalls)
			assistant := model.AIMessage{ID: newUUID(), TenantID: tenantID, ConversationID: conversation.ID, Role: "assistant", Content: answer, Sources: sourceJSON, ToolCalls: toolJSON, InputTokens: inputTokens, OutputTokens: outputTokens, CreatedAt: now}
			if err := tx.Create(&assistant).Error; err != nil {
				return err
			}
			result = model.AIAnswer{ConversationID: conversation.ID, Message: assistant, Sources: sources}
			return nil
		})
	})
	return result, err
}
func (r *AgentRepository) ListConversations(ctx context.Context, tenantID, userID string) ([]model.AIConversation, error) {
	result := []model.AIConversation{}
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		return db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Order("updated_at DESC").Limit(100).Find(&result).Error
	})
	return result, err
}
func (r *AgentRepository) ListMessages(ctx context.Context, tenantID, userID, conversationID string) ([]model.AIMessage, error) {
	result := []model.AIMessage{}
	err := r.knowledge.withDB(ctx, func(db *gorm.DB) error {
		var count int64
		if err := db.Model(&model.AIConversation{}).Where("tenant_id = ? AND user_id = ? AND id = ?", tenantID, userID, conversationID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrConversationNotFound
		}
		return db.Where("tenant_id = ? AND conversation_id = ?", tenantID, conversationID).Order("created_at").Find(&result).Error
	})
	return result, err
}
