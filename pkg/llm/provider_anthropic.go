package llm

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// AnthropicProvider implements Provider using the Anthropic Messages API.
type AnthropicProvider struct {
	Client *anthropic.Client
	Model  anthropic.Model
}

func (p AnthropicProvider) Complete(ctx context.Context, req ChatRequest) (model.Message, error) {
	if p.Client == nil {
		return model.Message{}, errors.New("anthropic client is required")
	}

	modelName := p.Model
	if modelName == "" {
		modelName = anthropic.ModelClaude3_7SonnetLatest
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
		return model.Message{}, errors.New("no messages to send")
	}

	var systemBlocks []anthropic.TextBlockParam
	if req.SystemPrompt != "" {
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: req.SystemPrompt,
			Type: constant.ValueOf[constant.Text](),
		})
	}

	resp, err := p.Client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     modelName,
		MaxTokens: 1024,
		Messages:  messages,
		System:    systemBlocks,
	})
	if err != nil {
		return model.Message{}, err
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

	return model.Message{
		Role:      model.RoleAssistant,
		Content:   content,
		Timestamp: time.Now(),
	}, nil
}

func toAnthropicMessage(msg model.Message) (anthropic.MessageParam, bool) {
	switch msg.Role {
	case model.RoleUser:
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)), true
	case model.RoleAssistant:
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
