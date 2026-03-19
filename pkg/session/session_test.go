package session

import (
	"context"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

type stubLLM struct {
	requests []llm.ChatRequest
}

func (s *stubLLM) Complete(ctx context.Context, req llm.ChatRequest) (llm.Message, error) {
	s.requests = append(s.requests, req)
	return llm.Message{
		Role:      llm.RoleAssistant,
		Content:   "reply: " + req.Prompt.Content,
		Timestamp: time.Unix(1, 0),
	}, nil
}

func TestSessionCompleteTracksTranscript(t *testing.T) {
	ctx := context.Background()
	provider := &stubLLM{}
	seedTranscript := []llm.Message{
		{Role: llm.RoleAssistant, Content: "seed"},
	}

	session := NewSession(provider, WithTranscript(seedTranscript))

	first, err := session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "hello"},
	})
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
	if len(req1.Transcript) != len(seedTranscript) {
		t.Fatalf("expected seed transcript forwarded, got %d messages", len(req1.Transcript))
	}
	if req1.Prompt.Role != llm.RoleUser {
		t.Fatalf("prompt role not preserved, got %s", req1.Prompt.Role)
	}
	if req1.Prompt.Timestamp.IsZero() {
		t.Fatalf("prompt timestamp should be set")
	}
	if got := len(session.Transcript()); got != 3 { // seed + prompt + reply
		t.Fatalf("unexpected transcript length after first exchange: %d", got)
	}

	second, err := session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "again"},
	})
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
	expectedRoles := []llm.Role{llm.RoleAssistant, llm.RoleUser, llm.RoleAssistant}
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
	_, err = session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "after reset"},
	})
	if err != nil {
		t.Fatalf("Complete after reset returned error: %v", err)
	}

	req3 := provider.requests[2]
	if len(req3.Transcript) != len(seedTranscript) {
		t.Fatalf("expected transcript reset to seed, got %d messages", len(req3.Transcript))
	}
}

func TestNoAutoCompression(t *testing.T) {
	ctx := context.Background()
	provider := &stubLLM{}
	sess := NewSession(provider)

	// Accumulate many messages - no automatic compression should occur
	for i := 0; i < 20; i++ {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: "msg"},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	// 20 exchanges = 40 messages (user + assistant each), no compression
	if got := len(sess.Transcript()); got != 40 {
		t.Fatalf("expected 40 messages without compression, got %d", got)
	}

	// All requests should be normal completions, no extra compression calls
	if len(provider.requests) != 20 {
		t.Fatalf("expected 20 requests (no compression calls), got %d", len(provider.requests))
	}
}
