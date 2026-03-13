package llm

import (
	"context"
	"errors"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// ChatRequest captures a normalized chat request for LLM providers.
type ChatRequest struct {
	SystemPrompt string
	Transcript   []model.Message
	Prompt       model.Message
}

// Provider abstracts a chat completion backend (OpenAI, Anthropic, etc).
type Provider interface {
	Complete(ctx context.Context, req ChatRequest) (model.Message, error)
}

// Agent adapts a Provider to the agent.Agent interface.
type Agent struct {
	Provider     Provider
	SystemPrompt string
}

var ErrProviderMissing = errors.New("chat provider is required")

func (a Agent) Respond(ctx context.Context, input agent.Input) (model.Message, error) {
	if a.Provider == nil {
		return model.Message{}, ErrProviderMissing
	}

	reply, err := a.Provider.Complete(ctx, ChatRequest{
		SystemPrompt: a.SystemPrompt,
		Transcript:   input.Transcript,
		Prompt: model.Message{
			Role:    model.RoleUser,
			Content: input.Event.Content,
		},
	})
	if err != nil {
		return model.Message{}, err
	}

	if reply.Role == "" {
		reply.Role = model.RoleAssistant
	}
	if reply.Timestamp.IsZero() {
		reply.Timestamp = time.Now()
	}
	return reply, nil
}
