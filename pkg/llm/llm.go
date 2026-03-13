package llm

import (
	"context"
	"time"
)

// ChatRequest captures a normalized chat request for LLM providers.
type ChatRequest struct {
	SystemPrompt string
	Prompt       Message
	Transcript   []Message
}

// provider abstracts a chat completion backend (OpenAI, Anthropic, etc).
type LLM interface {
	Complete(ctx context.Context, req ChatRequest) (Message, error)
}

// Role represents the speaker of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message is a transcript entry exchanged within a session.
type Message struct {
	Role      Role
	Content   string
	Timestamp time.Time
}
