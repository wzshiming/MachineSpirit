package llm

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
)

// ollamaProvider implements Provider using Ollama's official Go client.
type ollamaProvider struct {
	Client *api.Client
	Model  string
}

func (p ollamaProvider) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	if p.Client == nil {
		return Message{}, errors.New("ollama client is required")
	}
	if p.Model == "" {
		return Message{}, errors.New("model name is required")
	}

	var messages []api.Message
	if req.SystemPrompt != "" {
		messages = append(messages, api.Message{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	for _, msg := range req.Transcript {
		if omsg, ok := toOllamaMessage(msg); ok {
			messages = append(messages, omsg)
		}
	}
	if omsg, ok := toOllamaMessage(req.Prompt); ok {
		messages = append(messages, omsg)
	}
	if len(messages) == 0 {
		return Message{}, errors.New("no messages to send")
	}

	stream := false
	chatReq := &api.ChatRequest{
		Model:    p.Model,
		Messages: messages,
		Stream:   &stream,
	}

	var response api.ChatResponse
	err := p.Client.Chat(ctx, chatReq, func(resp api.ChatResponse) error {
		response = resp
		return nil
	})
	if err != nil {
		return Message{}, err
	}

	timestamp := time.Now()
	if !response.CreatedAt.IsZero() {
		timestamp = response.CreatedAt
	}

	return Message{
		Role:      RoleAssistant,
		Content:   response.Message.Content,
		Timestamp: timestamp,
	}, nil
}

func toOllamaMessage(msg Message) (api.Message, bool) {
	switch msg.Role {
	case RoleUser:
		return api.Message{Role: "user", Content: msg.Content}, true
	case RoleAssistant:
		return api.Message{Role: "assistant", Content: msg.Content}, true
	case RoleSystem:
		return api.Message{Role: "system", Content: msg.Content}, true
	default:
		return api.Message{}, false
	}
}

// newOllamaClient creates a new Ollama client with the provided base URL.
func newOllamaClient(baseURL string) (*api.Client, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: 300 * time.Second, // Ollama can be slow for large models
	}
	client := api.NewClient(u, httpClient)
	return client, nil
}
