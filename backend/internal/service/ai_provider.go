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
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

var ErrAIProviderUnavailable = errors.New("AI provider is not configured")

type AIProvider interface {
	Embed(context.Context, []string) ([][]float32, error)
	Complete(context.Context, string, string) (string, int, int, error)
}
type openAIProvider struct {
	config          config.AIConfig
	client          *http.Client
	tokens          metric.Int64Counter
	cost            metric.Float64Counter
	providerLatency metric.Float64Histogram
}

func NewAIProvider(cfg config.AIConfig) AIProvider {
	meter := otel.Meter("nexaflow/ai")
	tokens, _ := meter.Int64Counter("nexaflow.ai.tokens")
	cost, _ := meter.Float64Counter("nexaflow.ai.cost", metric.WithUnit("USD"))
	providerLatency, _ := meter.Float64Histogram("nexaflow.ai.provider.duration", metric.WithUnit("s"))
	return &openAIProvider{config: cfg, client: &http.Client{Timeout: cfg.Timeout}, tokens: tokens, cost: cost, providerLatency: providerLatency}
}

func (p *openAIProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, span := otel.Tracer("nexaflow/ai").Start(ctx, "ai.embed")
	defer span.End()
	span.SetAttributes(attribute.String("ai.model", p.config.EmbeddingModel), attribute.Int("ai.input.count", len(texts)))
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
	ctx, span := otel.Tracer("nexaflow/ai").Start(ctx, "ai.complete")
	defer span.End()
	span.SetAttributes(attribute.String("ai.model", p.config.ChatModel))
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", 0, 0, err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", 0, 0, fmt.Errorf("AI provider returned an empty response")
	}
	p.tokens.Add(ctx, int64(response.Usage.PromptTokens), metric.WithAttributes(attribute.String("ai.model", p.config.ChatModel), attribute.String("ai.token.type", "input")))
	p.tokens.Add(ctx, int64(response.Usage.CompletionTokens), metric.WithAttributes(attribute.String("ai.model", p.config.ChatModel), attribute.String("ai.token.type", "output")))
	estimatedCost := float64(response.Usage.PromptTokens)*p.config.InputCostPerMillionTokens/1_000_000 + float64(response.Usage.CompletionTokens)*p.config.OutputCostPerMillionTokens/1_000_000
	p.cost.Add(ctx, estimatedCost, metric.WithAttributes(attribute.String("ai.model", p.config.ChatModel)))
	span.SetAttributes(attribute.Int("ai.usage.input_tokens", response.Usage.PromptTokens), attribute.Int("ai.usage.output_tokens", response.Usage.CompletionTokens), attribute.Float64("ai.usage.cost_usd", estimatedCost))
	return response.Choices[0].Message.Content, response.Usage.PromptTokens, response.Usage.CompletionTokens, nil
}
func (p *openAIProvider) call(ctx context.Context, path string, input, output any) error {
	started := time.Now()
	defer func() {
		p.providerLatency.Record(ctx, time.Since(started).Seconds(), metric.WithAttributes(attribute.String("ai.provider", "openai-compatible")))
	}()
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
