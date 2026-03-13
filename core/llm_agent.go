package core

import (
	"context"
	"errors"
	"time"
)

var ErrProviderMissing = errors.New("chat provider is required")

// ChatRequest captures a normalized chat request for LLM providers.
type ChatRequest struct {
	SystemPrompt string
	Transcript   []Message
	Prompt       Message
}

// ChatProvider abstracts a chat completion backend (OpenAI, Anthropic, etc).
type ChatProvider interface {
	Complete(ctx context.Context, req ChatRequest) (Message, error)
}

// LLMAgent adapts a ChatProvider to the Agent interface.
type LLMAgent struct {
	Provider     ChatProvider
	SystemPrompt string
}

func (a LLMAgent) Respond(ctx context.Context, input AgentInput) (Message, error) {
	if a.Provider == nil {
		return Message{}, ErrProviderMissing
	}

	reply, err := a.Provider.Complete(ctx, ChatRequest{
		SystemPrompt: a.SystemPrompt,
		Transcript:   input.Transcript,
		Prompt: Message{
			Role:    RoleUser,
			Content: input.Event.Content,
		},
	})
	if err != nil {
		return Message{}, err
	}

	if reply.Role == "" {
		reply.Role = RoleAssistant
	}
	if reply.Timestamp.IsZero() {
		reply.Timestamp = time.Now()
	}
	return reply, nil
}
