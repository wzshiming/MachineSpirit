package llm

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
)

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
