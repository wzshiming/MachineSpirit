package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ollamaProvider implements Provider using Ollama's chat completion API.
// Ollama uses an OpenAI-compatible API format but runs locally.
type ollamaProvider struct {
	Client  *http.Client
	BaseURL string
	Model   string
}

// ollamaChatRequest represents the request to Ollama's /api/chat endpoint.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

// ollamaMessage represents a message in Ollama's chat format.
type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse represents the response from Ollama's /api/chat endpoint.
type ollamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

func (p ollamaProvider) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	if p.Client == nil {
		return Message{}, errors.New("http client is required")
	}
	if p.Model == "" {
		return Message{}, errors.New("model name is required")
	}

	var messages []ollamaMessage
	if req.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{
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

	reqBody := ollamaChatRequest{
		Model:    p.Model,
		Messages: messages,
		Stream:   false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Message{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := p.BaseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return Message{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return Message{}, fmt.Errorf("failed to decode response: %w", err)
	}

	timestamp := time.Now()
	if chatResp.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, chatResp.CreatedAt); err == nil {
			timestamp = t
		}
	}

	return Message{
		Role:      RoleAssistant,
		Content:   chatResp.Message.Content,
		Timestamp: timestamp,
	}, nil
}

func toOllamaMessage(msg Message) (ollamaMessage, bool) {
	switch msg.Role {
	case RoleUser:
		return ollamaMessage{Role: "user", Content: msg.Content}, true
	case RoleAssistant:
		return ollamaMessage{Role: "assistant", Content: msg.Content}, true
	case RoleSystem:
		return ollamaMessage{Role: "system", Content: msg.Content}, true
	default:
		return ollamaMessage{}, false
	}
}

// newOllamaClient creates a new HTTP client for Ollama API requests.
func newOllamaClient() *http.Client {
	return &http.Client{
		Timeout: 300 * time.Second, // Ollama can be slow for large models
	}
}
