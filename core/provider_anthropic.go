package core

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// AnthropicProvider implements ChatProvider using the Anthropic Messages API.
type AnthropicProvider struct {
	Client *anthropic.Client
	Model  anthropic.Model
}

func (p AnthropicProvider) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	if p.Client == nil {
		return Message{}, errors.New("anthropic client is required")
	}

	model := p.Model
	if model == "" {
		model = anthropic.ModelClaude3_7SonnetLatest
	}

	var messages []anthropic.MessageParam
	for _, msg := range req.Transcript {
		if param, ok := toAnthropicMessage(msg); ok {
			messages = append(messages, param)
		}
	}
	if param, ok := toAnthropicMessage(req.Prompt); ok {
		messages = append(messages, param)
	}
	if len(messages) == 0 {
		return Message{}, errors.New("no messages to send")
	}

	var systemBlocks []anthropic.TextBlockParam
	if req.SystemPrompt != "" {
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: req.SystemPrompt,
			Type: constant.ValueOf[constant.Text](),
		})
	}

	resp, err := p.Client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 1024,
		Messages:  messages,
		System:    systemBlocks,
	})
	if err != nil {
		return Message{}, err
	}
	content := ""
	for _, block := range resp.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			content = b.Text
			break
		}
		if content != "" {
			break
		}
	}

	return Message{
		Role:      RoleAssistant,
		Content:   content,
		Timestamp: time.Now(),
	}, nil
}

func toAnthropicMessage(msg Message) (anthropic.MessageParam, bool) {
	switch msg.Role {
	case RoleUser:
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)), true
	case RoleAssistant:
		return anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)), true
	default:
		return anthropic.MessageParam{}, false
	}
}

// NewAnthropicClient builds a client with the provided API key and optional base URL.
func NewAnthropicClient(apiKey string, baseURL string) *anthropic.Client {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)
	return &client
}
