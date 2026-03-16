package session

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

// Session tracks conversation state across multiple LLM completions.
type Session struct {
	llm            llm.LLM
	systemPrompt   string
	transcript     []llm.Message
	baseTranscript []llm.Message
}

type opt func(*Session)

// WithSystemPrompt sets a system prompt that is included in every LLM request.
func WithSystemPrompt(prompt string) opt {
	return func(s *Session) {
		s.systemPrompt = prompt
	}
}

// WithTranscript initializes the session with a seed transcript. This can be used to provide context or examples for the conversation. The seed transcript is preserved and can be reset to with the Reset() method.
func WithTranscript(transcript []llm.Message) opt {
	return func(s *Session) {
		s.transcript = append([]llm.Message(nil), transcript...)
		s.baseTranscript = append([]llm.Message(nil), transcript...)
	}
}

// NewSession creates a new Session bound to the provided LLM.
func NewSession(l llm.LLM, opts ...opt) *Session {
	s := &Session{
		llm: l,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Complete sends the prompt through the underlying LLM and records the exchange.
func (s *Session) Complete(ctx context.Context, req llm.ChatRequest) (llm.Message, error) {
	if s.llm == nil {
		return llm.Message{}, errors.New("llm provider is required")
	}

	os.Stderr.WriteString("=== LLM Request ===\n")
	os.Stderr.WriteString(req.Prompt.Content + "\n")
	os.Stderr.WriteString("====================\n")

	var history []llm.Message
	if s.transcript != nil {
		history = append(history, s.transcript...)
	}
	if req.Transcript != nil {
		history = append(history, req.Transcript...)
	}

	systemPrompt := s.systemPrompt
	if req.SystemPrompt != "" {
		systemPrompt = req.SystemPrompt
	}
	prompt := req.Prompt
	if prompt.Role == "" {
		prompt.Role = llm.RoleUser
	}
	if prompt.Timestamp.IsZero() {
		prompt.Timestamp = time.Now()
	}
	resp, err := s.llm.Complete(ctx, llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Transcript:   history,
		Prompt:       prompt,
	})
	if err != nil {
		return llm.Message{}, err
	}

	os.Stderr.WriteString("=== LLM Response ===\n")
	os.Stderr.WriteString(resp.Content + "\n")
	os.Stderr.WriteString("====================\n")

	s.transcript = append(s.transcript, prompt, resp)
	return resp, nil
}

// Transcript returns the current conversation history.
func (s *Session) Transcript() []llm.Message {
	return append([]llm.Message(nil), s.transcript...)
}

// Reset clears the conversation history, keeping the initial seed transcript.
func (s *Session) Reset() {
	s.transcript = append([]llm.Message(nil), s.baseTranscript...)
}
