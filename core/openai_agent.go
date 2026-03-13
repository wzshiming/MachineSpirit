package core

import (
	"context"
	"errors"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

// OpenAIAgent adapts the OpenAI chat completion client to the Agent interface.
type OpenAIAgent struct {
	Client       *openai.Client
	Model        shared.ChatModel
	SystemPrompt string
}

func (a OpenAIAgent) Respond(ctx context.Context, input AgentInput) (Message, error) {
	if a.Client == nil {
		return Message{}, errors.New("openai client is required")
	}

	model := a.Model
	if model == "" {
		model = shared.ChatModelGPT4_1Mini
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(input.Transcript)+2)
	if a.SystemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessageParamUnion{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Role:    constant.ValueOf[constant.System](),
				Content: openai.ChatCompletionSystemMessageParamContentUnion{OfString: openai.String(a.SystemPrompt)},
			},
		})
	}
	for _, msg := range input.Transcript {
		if mapped, ok := toOpenAIMessage(msg); ok {
			messages = append(messages, mapped)
		}
	}
	if mapped, ok := toOpenAIMessage(Message{
		Role:    RoleUser,
		Content: input.Event.Content,
	}); ok {
		messages = append(messages, mapped)
	}

	if len(messages) == 0 {
		return Message{}, errors.New("no messages to send")
	}

	resp, err := a.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    model,
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
	content := msg.Content
	switch msg.Role {
	case RoleAssistant:
		return openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				Role:    constant.ValueOf[constant.Assistant](),
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(content)},
			},
		}, true
	case RoleUser:
		return openai.ChatCompletionMessageParamUnion{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Role:    constant.ValueOf[constant.User](),
				Content: openai.ChatCompletionUserMessageParamContentUnion{OfString: openai.String(content)},
			},
		}, true
	case RoleSystem:
		return openai.ChatCompletionMessageParamUnion{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Role:    constant.ValueOf[constant.System](),
				Content: openai.ChatCompletionSystemMessageParamContentUnion{OfString: openai.String(content)},
			},
		}, true
	default:
		return openai.ChatCompletionMessageParamUnion{}, false
	}
}

// NewOpenAIClient builds a client with the provided API key and optional base URL.
func NewOpenAIClient(apiKey string, baseURL string) *openai.Client {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &client
}
