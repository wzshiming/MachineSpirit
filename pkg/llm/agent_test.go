package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type scriptedLLM struct {
	requests []ChatRequest
}

func (s *scriptedLLM) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	s.requests = append(s.requests, req)
	switch len(s.requests) {
	case 1:
		return Message{
			Role:      RoleAssistant,
			Content:   `<response>{"action":"call_tool","tool":"search","input":"New York to London tomorrow"}</response>`,
			Timestamp: time.Unix(1, 0),
		}, nil
	case 2:
		return Message{
			Role:      RoleAssistant,
			Content:   `{"action":"respond","reply":"Booked the evening flight for tomorrow."}`,
			Timestamp: time.Unix(2, 0),
		}, nil
	default:
		return Message{}, errors.New("unexpected call count")
	}
}

func TestAgentRunInvokesToolThenResponds(t *testing.T) {
	ctx := context.Background()
	llm := &scriptedLLM{}
	session := NewSession(llm, SessionConfig{SystemPrompt: "Be concise"})

	var toolInput string
	agent := NewAgent(session, AgentConfig{
		Tools: []Tool{
			{
				Name:        "search",
				Short:       "Flight search",
				Description: "Finds flights between locations on specified dates.",
				Parameters:  "text like \"NYC to LON tomorrow\"",
				Fn: func(ctx context.Context, input string) (string, error) {
					toolInput = input
					return "Found 1 option at 8pm, $500", nil
				},
			},
		},
		MaxSteps: 4,
	})

	resp, err := agent.Run(ctx, "Help me book a flight ticket from New York to London for tomorrow")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if toolInput != "New York to London tomorrow" {
		t.Fatalf("tool input mismatch: %q", toolInput)
	}
	if resp.Content != "Booked the evening flight for tomorrow." {
		t.Fatalf("unexpected final response: %q", resp.Content)
	}

	if len(llm.requests) != 2 {
		t.Fatalf("expected 2 llm calls, got %d", len(llm.requests))
	}
	sysPrompt := llm.requests[0].SystemPrompt
	if !strings.Contains(sysPrompt, "Tools you can call") ||
		!strings.Contains(sysPrompt, "search — Flight search") ||
		!strings.Contains(sysPrompt, "Details: Finds flights between locations on specified dates.") ||
		!strings.Contains(sysPrompt, "Parameters: text like") {
		t.Fatalf("system prompt missing tool descriptions: %q", sysPrompt)
	}

	if got := len(session.Transcript()); got != 4 {
		t.Fatalf("expected transcript length 4, got %d", got)
	}
}

type simpleLLM struct {
	requests []ChatRequest
}

func (s *simpleLLM) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	s.requests = append(s.requests, req)
	return Message{
		Role:      RoleAssistant,
		Content:   "Direct answer without tools.",
		Timestamp: time.Unix(int64(len(s.requests)), 0),
	}, nil
}

func TestAgentRunFallsBackToDirectResponse(t *testing.T) {
	ctx := context.Background()
	llm := &simpleLLM{}
	session := NewSession(llm, SessionConfig{})
	agent := NewAgent(session, AgentConfig{})

	resp, err := agent.Run(ctx, "Just say hi")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if resp.Content != "Direct answer without tools." {
		t.Fatalf("unexpected response: %q", resp.Content)
	}

	if got := len(session.Transcript()); got != 2 {
		t.Fatalf("expected transcript length 2, got %d", got)
	}
}
