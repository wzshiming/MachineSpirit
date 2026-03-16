package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
)

// ChatRequest captures a normalized chat request for LLM providers.
type ChatRequest struct {
	SystemPrompt string
	Prompt       Message
	Transcript   []Message
}

// LLM defines the interface for language model providers. It abstracts away provider-specific details and allows for interchangeable implementations.
type LLM interface {
	Complete(ctx context.Context, req ChatRequest) (Message, error)
}

// Role represents the speaker of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message is a transcript entry exchanged within a session.
type Message struct {
	Role      Role
	Content   string
	Timestamp time.Time
}

// Config describes how to construct an LLM provider and agent.
type options struct {
	Provider string // "openai" or "anthropic"
	Model    string
	APIKey   string
	BaseURL  string
}

type opt func(*options)

func WithProvider(provider string) opt {
	return func(o *options) {
		o.Provider = provider
	}
}

func WithModel(model string) opt {
	return func(o *options) {
		o.Model = model
	}
}

func WithAPIKey(apiKey string) opt {
	return func(o *options) {
		o.APIKey = apiKey
	}
}

func WithBaseURL(baseURL string) opt {
	return func(o *options) {
		o.BaseURL = baseURL
	}
}

// NewLLM builds an LLM based on the supplied configuration.
func NewLLM(opts ...opt) (LLM, error) {
	var cfg options
	for _, apply := range opts {
		apply(&cfg)
	}

	switch strings.ToLower(cfg.Provider) {
	case "", "openai":
		client := newOpenAIClient(cfg.APIKey, cfg.BaseURL)
		modelName := cfg.Model
		if modelName == "" {
			modelName = string(openai.ChatModelGPT5)
		}
		return openAIProvider{
			Client: client,
			Model:  openai.ChatModel(modelName),
		}, nil
	case "anthropic":
		client := newAnthropicClient(cfg.APIKey, cfg.BaseURL)
		modelName := cfg.Model
		if modelName == "" {
			modelName = string(anthropic.ModelClaudeOpus4_5)
		}
		return anthropicProvider{
			Client: client,
			Model:  anthropic.Model(modelName),
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
