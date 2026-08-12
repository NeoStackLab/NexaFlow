package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type agentRepositoryStub struct{ toolCalls []map[string]any }

func (s *agentRepositoryStub) SaveExchange(_ context.Context, _, _ string, _ model.AIAskInput, answer string, sources []model.KnowledgeSource, tools []map[string]any, _, _ int) (model.AIAnswer, error) {
	s.toolCalls = tools
	return model.AIAnswer{Message: model.AIMessage{Content: answer}, Sources: sources}, nil
}
func (*agentRepositoryStub) ListConversations(context.Context, string, string) ([]model.AIConversation, error) {
	return nil, nil
}
func (*agentRepositoryStub) ListMessages(context.Context, string, string, string) ([]model.AIMessage, error) {
	return nil, nil
}

type knowledgeServiceStub struct{ searches int }

func (*knowledgeServiceStub) Ingest(context.Context, string, string, string, string, int64, io.Reader) (model.KnowledgeDocument, error) {
	return model.KnowledgeDocument{}, nil
}
func (*knowledgeServiceStub) List(context.Context, string) ([]model.KnowledgeDocument, error) {
	return nil, nil
}
func (*knowledgeServiceStub) Delete(context.Context, string, string) error { return nil }
func (s *knowledgeServiceStub) Search(context.Context, string, string, int) ([]model.KnowledgeSource, error) {
	s.searches++
	return []model.KnowledgeSource{{DocumentName: "Policy", Content: "Approved"}}, nil
}

type providerStub struct{ completions int }

func (*providerStub) Embed(context.Context, []string) ([][]float32, error) { return nil, nil }
func (s *providerStub) Complete(context.Context, string, string) (string, int, int, error) {
	s.completions++
	return "Answer [S1]", 10, 3, nil
}

func TestAgentRequiresKnowledgePermissionBeforeTools(t *testing.T) {
	repository, knowledge, provider := &agentRepositoryStub{}, &knowledgeServiceStub{}, &providerStub{}
	service := NewAgentService(repository, knowledge, provider, &modelServiceStub{}, &recordRepositoryServiceStub{})
	_, err := service.Ask(context.Background(), "tenant-a", "user-1", nil, model.AIAskInput{Message: "Question"})
	if !errors.Is(err, ErrInvalidAIRequest) || knowledge.searches != 0 || provider.completions != 0 {
		t.Fatalf("error=%v searches=%d completions=%d", err, knowledge.searches, provider.completions)
	}
}

type recordRepositoryServiceStub struct{}

func (*recordRepositoryServiceStub) Create(context.Context, string, string, string, model.WriteRecordInput) (model.RecordView, error) {
	return model.RecordView{}, nil
}
func (*recordRepositoryServiceStub) List(context.Context, string, string, int, int) ([]model.RecordView, int64, error) {
	return nil, 0, nil
}
func (*recordRepositoryServiceStub) Get(context.Context, string, string, string) (model.RecordView, error) {
	return model.RecordView{}, nil
}
func (*recordRepositoryServiceStub) Update(context.Context, string, string, string, string, model.WriteRecordInput) (model.RecordView, error) {
	return model.RecordView{}, nil
}
func (*recordRepositoryServiceStub) Delete(context.Context, string, string, string, int) error {
	return nil
}

func TestAgentAuditsKnowledgeToolWithoutRecordPermission(t *testing.T) {
	repository, knowledge, provider := &agentRepositoryStub{}, &knowledgeServiceStub{}, &providerStub{}
	service := NewAgentService(repository, knowledge, provider, &modelServiceStub{}, &recordRepositoryServiceStub{})
	result, err := service.Ask(context.Background(), "tenant-a", "user-1", []string{"knowledge.search"}, model.AIAskInput{Message: "What is policy?"})
	if err != nil || result.Message.Content == "" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if len(repository.toolCalls) != 1 || repository.toolCalls[0]["tool"] != "knowledge.search" {
		t.Fatalf("tools=%#v", repository.toolCalls)
	}
}
