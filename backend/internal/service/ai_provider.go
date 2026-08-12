package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
)

var ErrAIProviderUnavailable = errors.New("AI provider is not configured")

type AIProvider interface {
	Embed(context.Context, []string) ([][]float32, error)
	Complete(context.Context, string, string) (string, int, int, error)
}
type openAIProvider struct {
	config config.AIConfig
	client *http.Client
}

func NewAIProvider(cfg config.AIConfig) AIProvider {
	return &openAIProvider{config: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (p *openAIProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if strings.TrimSpace(p.config.APIKey) == "" {
		return nil, ErrAIProviderUnavailable
	}
	var response struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := p.call(ctx, "/embeddings", map[string]any{"model": p.config.EmbeddingModel, "input": texts, "dimensions": 1536}, &response); err != nil {
		return nil, err
	}
	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("embedding provider returned %d vectors for %d inputs", len(response.Data), len(texts))
	}
	result := make([][]float32, len(texts))
	for _, item := range response.Data {
		if item.Index < 0 || item.Index >= len(result) || len(item.Embedding) != 1536 {
			return nil, fmt.Errorf("embedding provider returned invalid vector")
		}
		result[item.Index] = item.Embedding
	}
	return result, nil
}
func (p *openAIProvider) Complete(ctx context.Context, system, user string) (string, int, int, error) {
	if strings.TrimSpace(p.config.APIKey) == "" {
		return "", 0, 0, ErrAIProviderUnavailable
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := p.call(ctx, "/chat/completions", map[string]any{"model": p.config.ChatModel, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}}, &response); err != nil {
		return "", 0, 0, err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", 0, 0, fmt.Errorf("AI provider returned an empty response")
	}
	return response.Choices[0].Message.Content, response.Usage.PromptTokens, response.Usage.CompletionTokens, nil
}
func (p *openAIProvider) call(ctx context.Context, path string, input, output any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.config.BaseURL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("AI provider request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 2048))
		return fmt.Errorf("AI provider returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(output); err != nil {
		return fmt.Errorf("decode AI provider response: %w", err)
	}
	return nil
}
