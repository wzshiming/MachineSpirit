package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/model"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

type providerConfig struct {
	Name    string
	Model   string
	APIKey  string
	BaseURL string
	System  string
}

func main() {
	cfg := parseFlags()

	provider, err := buildProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	agent := llm.Agent{
		Provider:     provider,
		SystemPrompt: cfg.System,
	}

	manager := session.NewManager(agent)

	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		env, err := manager.HandleEvent(ctx, model.Event{
			SessionID: "cli",
			Content:   text,
			Timestamp: time.Now(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		for _, msg := range env.Messages {
			fmt.Println(msg.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() providerConfig {
	var cfg providerConfig
	flag.StringVar(&cfg.Name, "provider", "openai", "LLM provider: openai or anthropic")
	flag.StringVar(&cfg.Model, "model", "", "Model name (optional, provider default used if empty)")
	flag.StringVar(&cfg.APIKey, "api-key", "", "API key for the provider (env fallback OPENAI_API_KEY or ANTHROPIC_API_KEY)")
	flag.StringVar(&cfg.BaseURL, "base-url", "", "Optional base URL for the provider API")
	flag.StringVar(&cfg.System, "system", "", "Optional system prompt")
	flag.Parse()

	if cfg.APIKey == "" {
		switch cfg.Name {
		case "openai":
			cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		case "anthropic":
			cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	}
	return cfg
}

func buildProvider(cfg providerConfig) (llm.Provider, error) {
	switch strings.ToLower(cfg.Name) {
	case "openai":
		client := llm.NewOpenAIClient(cfg.APIKey, cfg.BaseURL)
		modelName := cfg.Model
		if modelName == "" {
			modelName = "gpt-4.1-mini"
		}
		return llm.OpenAIProvider{
			Client: client,
			Model:  modelName,
		}, nil
	case "anthropic":
		client := llm.NewAnthropicClient(cfg.APIKey, cfg.BaseURL)
		modelName := cfg.Model
		if modelName == "" {
			modelName = "claude-3-7-sonnet-latest"
		}
		return llm.AnthropicProvider{
			Client: client,
			Model:  anthropic.Model(modelName),
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Name)
	}
}
