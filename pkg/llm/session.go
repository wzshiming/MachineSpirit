package llm

import (
	"context"
	"errors"
	"time"
)

// Session tracks conversation state across multiple LLM completions.
type Session struct {
	llm            LLM
	systemPrompt   string
	transcript     []Message
	baseTranscript []Message
}

// SessionConfig configures a new Session.
type SessionConfig struct {
	// SystemPrompt prepends a system message to every request.
	SystemPrompt string
	// Transcript seeds the conversation with an existing exchange.
	Transcript []Message
}

// NewSession creates a new Session bound to the provided LLM.
func NewSession(llm LLM, cfg SessionConfig) *Session {
	base := append([]Message(nil), cfg.Transcript...)
	return &Session{
		llm:            llm,
		systemPrompt:   cfg.SystemPrompt,
		transcript:     append([]Message(nil), base...),
		baseTranscript: base,
	}
}

// Complete sends the prompt through the underlying LLM and records the exchange.
func (s *Session) Complete(ctx context.Context, prompt Message) (Message, error) {
	return s.complete(ctx, prompt, s.systemPrompt)
}

func (s *Session) complete(ctx context.Context, prompt Message, systemPrompt string) (Message, error) {
	if s.llm == nil {
		return Message{}, errors.New("llm provider is required")
	}

	if prompt.Timestamp.IsZero() {
		prompt.Timestamp = time.Now()
	}

	history := append([]Message(nil), s.transcript...)

	resp, err := s.llm.Complete(ctx, ChatRequest{
		SystemPrompt: systemPrompt,
		Transcript:   history,
		Prompt:       prompt,
	})
	if err != nil {
		return Message{}, err
	}

	s.transcript = append(s.transcript, prompt, resp)
	return resp, nil
}

// Transcript returns the current conversation history.
func (s *Session) Transcript() []Message {
	return append([]Message(nil), s.transcript...)
}

// Reset clears the conversation history, keeping the initial seed transcript.
func (s *Session) Reset() {
	s.transcript = append([]Message(nil), s.baseTranscript...)
}
