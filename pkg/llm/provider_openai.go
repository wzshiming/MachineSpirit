package llm

import (
	"context"
	"errors"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// openAIProvider implements Provider using OpenAI chat completions.
type openAIProvider struct {
	Client *openai.Client
	Model  shared.ChatModel
}

func (p openAIProvider) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	if p.Client == nil {
		return Message{}, errors.New("openai client is required")
	}

	modelName := p.Model

	var messages []openai.ChatCompletionMessageParamUnion
	if req.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(req.SystemPrompt))
	}
	for _, msg := range req.Transcript {
		if param, ok := toOpenAIMessage(msg); ok {
			messages = append(messages, param)
		}
	}
	if param, ok := toOpenAIMessage(req.Prompt); ok {
		messages = append(messages, param)
	}
	if len(messages) == 0 {
		return Message{}, errors.New("no messages to send")
	}

	resp, err := p.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    modelName,
		Messages: messages,
	})
	if err != nil {
		return Message{}, err
	}
	if len(resp.Choices) == 0 {
		return Message{}, errors.New("no choices returned from openai")
	}

	choice := resp.Choices[0]
	timestamp := time.Now()
	if resp.Created > 0 {
		timestamp = time.Unix(resp.Created, 0)
	}

	return Message{
		Role:      RoleAssistant,
		Content:   choice.Message.Content,
		Timestamp: timestamp,
	}, nil
}

func toOpenAIMessage(msg Message) (openai.ChatCompletionMessageParamUnion, bool) {
	switch msg.Role {
	case RoleAssistant:
		return openai.ChatCompletionMessageParamOfAssistant(msg.Content), true
	case RoleUser:
		return openai.UserMessage(msg.Content), true
	case RoleSystem:
		return openai.SystemMessage(msg.Content), true
	default:
		return openai.ChatCompletionMessageParamUnion{}, false
	}
}

// newOpenAIClient builds a client with the provided API key and optional base URL.
func newOpenAIClient(apiKey string, baseURL string) *openai.Client {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &client
}
