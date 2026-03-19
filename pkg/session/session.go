package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
)

// minRecentMessages is the minimum number of recent messages to keep
// during compression, ensuring at least one user-assistant exchange
// remains in full for context continuity.
const minRecentMessages = 2

// Session tracks conversation state across multiple LLM completions.
type Session struct {
	llm            llm.LLM
	transcript     []llm.Message
	baseTranscript []llm.Message
	pm             *persistence.PersistenceManager
}

type opt func(*Session)

// WithTranscript initializes the session with a seed transcript. This can be used to provide context or examples for the conversation. The seed transcript is preserved and can be reset to with the Reset() method.
func WithTranscript(transcript []llm.Message) opt {
	return func(s *Session) {
		s.transcript = append([]llm.Message(nil), transcript...)
		s.baseTranscript = append([]llm.Message(nil), transcript...)
	}
}

// WithPersistenceManager sets the persistence manager for the agent.
func WithPersistenceManager(pm *persistence.PersistenceManager) opt {
	return func(s *Session) {
		s.pm = pm
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

	systemPrompt := req.SystemPrompt
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

// CompressTranscript reduces transcript size by summarizing older messages.
func (s *Session) CompressTranscript(ctx context.Context, keepRecent int, systemPrompt string) error {
	currentCount := len(s.transcript)
	if currentCount <= minRecentMessages {
		return fmt.Errorf("transcript too short to compress (minimum %d messages needed)", minRecentMessages)
	}

	// Determine how many recent messages to keep
	var keep int
	if keepRecent > 0 {
		keep = keepRecent
		if keep < minRecentMessages {
			keep = minRecentMessages
		}
		if keep >= currentCount {
			return fmt.Errorf("keep_recent (%d) must be less than current transcript size (%d)", keep, currentCount)
		}
	} else {
		// Default: keep half of current messages, minimum of 2
		keep = currentCount / 2
		if keep < minRecentMessages {
			keep = minRecentMessages
		}
	}

	// Determine what to compress: everything after base transcript up to the recent messages
	baseLen := len(s.baseTranscript)
	compressEnd := len(s.transcript) - keep
	if compressEnd <= baseLen {
		return fmt.Errorf("not enough messages to compress after base transcript")
	}

	toCompress := s.transcript[baseLen:compressEnd]
	recentMessages := s.transcript[compressEnd:]

	// Build the conversation text for summarization
	var sb strings.Builder
	for _, msg := range toCompress {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}

	// Ask the LLM to summarize the older messages
	summaryResp, err := s.llm.Complete(ctx, llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Prompt: llm.Message{
			Role:    llm.RoleUser,
			Content: sb.String(),
		},
	})
	if err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	// Rebuild transcript: base + summary + recent
	newTranscript := make([]llm.Message, 0, baseLen+1+len(recentMessages))
	newTranscript = append(newTranscript, s.baseTranscript...)
	newTranscript = append(newTranscript, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   summaryResp.Content,
		Timestamp: time.Now(),
	})
	newTranscript = append(newTranscript, recentMessages...)

	s.transcript = newTranscript
	return nil
}

// Size returns the number of messages in the current transcript.
func (s *Session) Size() int {
	return len(s.transcript)
}

// Transcript returns the current conversation history.
func (s *Session) Transcript() []llm.Message {
	return append([]llm.Message(nil), s.transcript...)
}

// AddMessages appends messages to the transcript without invoking the LLM.
// This is useful for feeding information from sub-agents or external sources
// into the conversation history.
func (s *Session) AddMessages(messages ...llm.Message) {
	for _, msg := range messages {
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now()
		}
		s.transcript = append(s.transcript, msg)
	}
}

// Reset clears the conversation history, keeping the initial seed transcript.
func (s *Session) Reset() {
	s.transcript = append([]llm.Message(nil), s.baseTranscript...)
}
