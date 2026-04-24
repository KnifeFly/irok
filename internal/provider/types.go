package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Interface interface {
	ListModels(ctx context.Context) ([]Model, error)
	CreateMessage(ctx context.Context, req MessageRequest) (MessageResponse, error)
	CreateMessageStream(ctx context.Context, req MessageRequest, emit func(StreamEvent) error) error
	CountTokens(ctx context.Context, req MessageRequest) (TokenCount, error)
	RefreshCredential(ctx context.Context, nodeID string) error
	UsageLimits(ctx context.Context, nodeID string) (map[string]any, error)
	HealthCheck(ctx context.Context, nodeID string) error
}

type Model struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

type MessageRequest struct {
	Model      string          `json:"model"`
	MaxTokens  int             `json:"max_tokens"`
	Messages   []Message       `json:"messages"`
	SystemText string          `json:"-"`
	Tools      []Tool          `json:"tools,omitempty"`
	Stream     bool            `json:"stream,omitempty"`
	Thinking   json.RawMessage `json:"thinking,omitempty"`
}

type RawMessageRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []Message       `json:"messages"`
	System    json.RawMessage `json:"system,omitempty"`
	Tools     []Tool          `json:"tools,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
	Thinking  json.RawMessage `json:"thinking,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
	Content      []ContentBlock `json:"content"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Input    any    `json:"input,omitempty"`
}

type StreamEvent struct {
	Type         string         `json:"type"`
	Message      *MessageStart  `json:"message,omitempty"`
	Index        *int           `json:"index,omitempty"`
	ContentBlock *ContentBlock  `json:"content_block,omitempty"`
	Delta        map[string]any `json:"delta,omitempty"`
	Usage        *Usage         `json:"usage,omitempty"`
}

type MessageStart struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Model   string         `json:"model"`
	Usage   Usage          `json:"usage"`
	Content []ContentBlock `json:"content"`
}

type TokenCount struct {
	InputTokens int `json:"input_tokens"`
}

func FromRaw(raw RawMessageRequest, systemText string) MessageRequest {
	return MessageRequest{
		Model:      raw.Model,
		MaxTokens:  raw.MaxTokens,
		Messages:   raw.Messages,
		SystemText: systemText,
		Tools:      raw.Tools,
		Stream:     raw.Stream,
		Thinking:   raw.Thinking,
	}
}

func NormalizeSystem(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, nil
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err == nil {
		parts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			if typ, _ := block["type"].(string); typ != "" && typ != "text" {
				continue
			}
			if text, _ := block["text"].(string); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n"), nil
	}
	return "", errors.New("system must be a string or an array of text blocks")
}

func TextFromContent(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, part := range v {
			parts = append(parts, TextFromContent(part))
		}
		return strings.Join(filterEmpty(parts), "\n")
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			return text
		}
		if nested, ok := v["content"]; ok {
			return TextFromContent(nested)
		}
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func EstimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	tokens := runes / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func filterEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}
