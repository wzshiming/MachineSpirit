package llm

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// Config describes how to construct an LLM provider and agent.
type Config struct {
	Provider     string // "openai" or "anthropic"
	Model        string
	APIKey       string
	BaseURL      string
	SystemPrompt string
}

// NewProvider builds a Provider based on the supplied configuration.
func NewProvider(cfg Config) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "", "openai":
		client := NewOpenAIClient(cfg.APIKey, cfg.BaseURL)
		modelName := cfg.Model
		if modelName == "" {
			modelName = string(openai.ChatModelGPT4_1Mini)
		}
		return OpenAIProvider{
			Client: client,
			Model:  openai.ChatModel(modelName),
		}, nil
	case "anthropic":
		client := NewAnthropicClient(cfg.APIKey, cfg.BaseURL)
		modelName := cfg.Model
		if modelName == "" {
			modelName = string(anthropic.ModelClaude3_7SonnetLatest)
		}
		return AnthropicProvider{
			Client: client,
			Model:  anthropic.Model(modelName),
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

// NewAgent constructs an llm.Agent using the configured provider and system prompt.
func NewAgent(cfg Config) (agent.Agent, error) {
	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	return Agent{
		Provider:     provider,
		SystemPrompt: cfg.SystemPrompt,
	}, nil
}
