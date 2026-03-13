package llm

import (
	"context"
	"testing"
	"time"
)

type stubLLM struct {
	requests []ChatRequest
}

func (s *stubLLM) Complete(ctx context.Context, req ChatRequest) (Message, error) {
	s.requests = append(s.requests, req)
	return Message{
		Role:      RoleAssistant,
		Content:   "reply: " + req.Prompt.Content,
		Timestamp: time.Unix(1, 0),
	}, nil
}

func TestSessionCompleteTracksTranscript(t *testing.T) {
	ctx := context.Background()
	provider := &stubLLM{}
	seedTranscript := []Message{
		{Role: RoleAssistant, Content: "seed"},
	}

	session := NewSession(provider, SessionConfig{
		SystemPrompt: "You are helpful",
		Transcript:   seedTranscript,
	})

	first, err := session.Complete(ctx, Message{Role: RoleUser, Content: "hello"})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if first.Content != "reply: hello" {
		t.Fatalf("unexpected first reply %q", first.Content)
	}

	if len(provider.requests) != 1 {
		t.Fatalf("expected 1 request recorded, got %d", len(provider.requests))
	}
	req1 := provider.requests[0]
	if req1.SystemPrompt != "You are helpful" {
		t.Fatalf("system prompt not forwarded, got %q", req1.SystemPrompt)
	}
	if len(req1.Transcript) != len(seedTranscript) {
		t.Fatalf("expected seed transcript forwarded, got %d messages", len(req1.Transcript))
	}
	if req1.Prompt.Role != RoleUser {
		t.Fatalf("prompt role not preserved, got %s", req1.Prompt.Role)
	}
	if req1.Prompt.Timestamp.IsZero() {
		t.Fatalf("prompt timestamp should be set")
	}
	if got := len(session.Transcript()); got != 3 { // seed + prompt + reply
		t.Fatalf("unexpected transcript length after first exchange: %d", got)
	}

	second, err := session.Complete(ctx, Message{Role: RoleUser, Content: "again"})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if second.Content != "reply: again" {
		t.Fatalf("unexpected second reply %q", second.Content)
	}

	if len(provider.requests) != 2 {
		t.Fatalf("expected 2 requests recorded, got %d", len(provider.requests))
	}
	req2 := provider.requests[1]
	if len(req2.Transcript) != len(seedTranscript)+2 {
		t.Fatalf("expected prior exchange forwarded, got %d messages", len(req2.Transcript))
	}
	expectedRoles := []Role{RoleAssistant, RoleUser, RoleAssistant}
	for i, role := range expectedRoles {
		if req2.Transcript[i].Role != role {
			t.Fatalf("unexpected role at %d: %s", i, req2.Transcript[i].Role)
		}
	}

	if got := len(session.Transcript()); got != 5 { // seed + 2 exchanges
		t.Fatalf("unexpected transcript length after second exchange: %d", got)
	}
	last := session.Transcript()[len(session.Transcript())-1]
	if last.Content != second.Content {
		t.Fatalf("last transcript message mismatch: %q", last.Content)
	}

	session.Reset()
	_, err = session.Complete(ctx, Message{Role: RoleUser, Content: "after reset"})
	if err != nil {
		t.Fatalf("Complete after reset returned error: %v", err)
	}

	req3 := provider.requests[2]
	if len(req3.Transcript) != len(seedTranscript) {
		t.Fatalf("expected transcript reset to seed, got %d messages", len(req3.Transcript))
	}
}
