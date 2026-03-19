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

func TestAddMessages(t *testing.T) {
	provider := &stubLLM{}
	sess := NewSession(provider)

	// Initially empty transcript
	if len(sess.Transcript()) != 0 {
		t.Fatalf("expected empty transcript, got %d messages", len(sess.Transcript()))
	}

	// Add some messages without invoking the LLM
	sess.AddMessages(
		llm.Message{Role: llm.RoleUser, Content: "external input 1"},
		llm.Message{Role: llm.RoleAssistant, Content: "external response 1"},
	)

	// Verify messages were added
	transcript := sess.Transcript()
	if len(transcript) != 2 {
		t.Fatalf("expected 2 messages after AddMessages, got %d", len(transcript))
	}
	if transcript[0].Content != "external input 1" {
		t.Errorf("unexpected first message content: %q", transcript[0].Content)
	}
	if transcript[1].Content != "external response 1" {
		t.Errorf("unexpected second message content: %q", transcript[1].Content)
	}

	// Verify timestamps are set
	if transcript[0].Timestamp.IsZero() {
		t.Error("expected timestamp to be set on first message")
	}
	if transcript[1].Timestamp.IsZero() {
		t.Error("expected timestamp to be set on second message")
	}

	// Verify no LLM calls were made
	if len(provider.requests) != 0 {
		t.Fatalf("expected no LLM requests, got %d", len(provider.requests))
	}

	// Add more messages
	sess.AddMessages(llm.Message{Role: llm.RoleUser, Content: "external input 2"})

	transcript = sess.Transcript()
	if len(transcript) != 3 {
		t.Fatalf("expected 3 messages after second AddMessages, got %d", len(transcript))
	}

	// Now do a normal Complete call
	ctx := context.Background()
	_, err := sess.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "normal prompt"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	// Verify the LLM received the previously added messages in the transcript
	if len(provider.requests) != 1 {
		t.Fatalf("expected 1 LLM request, got %d", len(provider.requests))
	}
	req := provider.requests[0]
	if len(req.Transcript) != 3 {
		t.Fatalf("expected 3 messages in transcript sent to LLM, got %d", len(req.Transcript))
	}
	if req.Transcript[0].Content != "external input 1" {
		t.Errorf("LLM didn't receive external message 1: %q", req.Transcript[0].Content)
	}

	// Final transcript should have all messages
	transcript = sess.Transcript()
	if len(transcript) != 5 { // 3 external + prompt + response
		t.Fatalf("expected 5 messages in final transcript, got %d", len(transcript))
	}
}
