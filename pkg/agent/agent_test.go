package agent

import (
	"context"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// stubAgentLLM is a minimal LLM for agent tests.
type stubAgentLLM struct{}

func (s *stubAgentLLM) Complete(_ context.Context, req llm.ChatRequest) (llm.Message, error) {
	return llm.Message{
		Role:      llm.RoleAssistant,
		Content:   "echo: " + req.Prompt.Content,
		Timestamp: time.Unix(1, 0),
	}, nil
}

func newTestAgent(t *testing.T) *Agent {
	t.Helper()
	provider := &stubAgentLLM{}
	sess := session.NewSession(provider)
	ag, err := NewAgent(sess)
	if err != nil {
		t.Fatalf("NewAgent failed: %v", err)
	}
	return ag
}

func TestAgentAddInputAndDrain(t *testing.T) {
	ag := newTestAgent(t)

	// Initially no pending inputs
	if ag.HasPendingInputs() {
		t.Error("expected no pending inputs initially")
	}
	msgs := ag.DrainInputs()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}

	// Add messages
	ag.AddInput(llm.Message{Role: llm.RoleUser, Content: "hello"})
	ag.AddInput(llm.Message{Role: llm.RoleUser, Content: "world"})

	if !ag.HasPendingInputs() {
		t.Error("expected pending inputs after AddInput")
	}

	// Drain and verify
	msgs = ag.DrainInputs()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("expected first message 'hello', got %q", msgs[0].Content)
	}
	if msgs[1].Content != "world" {
		t.Errorf("expected second message 'world', got %q", msgs[1].Content)
	}

	// After drain, queue should be empty
	if ag.HasPendingInputs() {
		t.Error("expected no pending inputs after drain")
	}
	msgs = ag.DrainInputs()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after second drain, got %d", len(msgs))
	}
}

func TestAgentAddInputConcurrent(t *testing.T) {
	ag := newTestAgent(t)

	const count = 50
	done := make(chan struct{})
	for i := range count {
		go func(n int) {
			ag.AddInput(llm.Message{
				Role:    llm.RoleUser,
				Content: "msg",
			})
			done <- struct{}{}
		}(i)
	}

	for range count {
		<-done
	}

	msgs := ag.DrainInputs()
	if len(msgs) != count {
		t.Errorf("expected %d messages, got %d", count, len(msgs))
	}
}
