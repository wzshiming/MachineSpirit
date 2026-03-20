package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

// stubLLM is a minimal LLM for session tests that echoes back the prompt.
type stubLLM struct {
	calls int // tracks how many times Complete was called
}

func (s *stubLLM) Complete(_ context.Context, req llm.ChatRequest) (llm.Message, error) {
	s.calls++
	return llm.Message{
		Role:      llm.RoleAssistant,
		Content:   "done: " + req.Prompt.Content,
		Timestamp: time.Unix(1, 0),
	}, nil
}

func TestCompressTranscriptUsesSubSession(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &stubLLM{}
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
		WithSave("sub-session-compress-test"),
	)

	ctx := context.Background()
	// Add 20 messages (10 exchanges)
	for i := range 10 {
		_, err := sess.Complete(ctx, SessionRequest{
			Prompt: Message{Role: RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	// Record state before compression
	beforeCount := sess.Size()
	if beforeCount != 20 {
		t.Fatalf("expected 20 messages before compression, got %d", beforeCount)
	}

	// Record the transcript content before compression to verify it's not polluted
	transcriptBefore := sess.Transcript()

	// Track LLM calls to verify the sub-session makes its own call
	callsBefore := provider.calls

	// Compress with keep_recent=4
	_, err := sess.CompressTranscript(ctx, 4, "Summarize this conversation.")
	if err != nil {
		t.Fatalf("CompressTranscript failed: %v", err)
	}

	// Verify compression happened: 1 summary + 4 recent = 5
	afterCount := sess.Size()
	if afterCount != 5 {
		t.Errorf("expected 5 messages after compression, got %d", afterCount)
	}

	// Verify the LLM was called exactly once more (by the sub-session)
	callsAfter := provider.calls
	if callsAfter-callsBefore != 1 {
		t.Errorf("expected exactly 1 additional LLM call for sub-session compression, got %d", callsAfter-callsBefore)
	}

	// Verify that the main session's transcript only contains:
	// [summary] + [last 4 messages from the original transcript]
	transcript := sess.Transcript()

	// First message should be the summary (assistant role)
	if transcript[0].Role != llm.RoleAssistant {
		t.Errorf("expected first message to be assistant (summary), got %v", transcript[0].Role)
	}

	// The remaining 4 messages should match the last 4 from the original transcript
	for i := 1; i < len(transcript); i++ {
		original := transcriptBefore[len(transcriptBefore)-4+i-1]
		if transcript[i].Content != original.Content {
			t.Errorf("message %d: expected %q, got %q", i, original.Content, transcript[i].Content)
		}
		if transcript[i].Role != original.Role {
			t.Errorf("message %d: expected role %v, got %v", i, original.Role, transcript[i].Role)
		}
	}

	// Verify the summary does NOT appear in the original transcript's messages
	// (i.e., the compression exchange was not added to the main session)
	summary := transcript[0].Content
	for _, msg := range transcriptBefore {
		if msg.Content == summary {
			t.Error("compression summary should not appear in the original transcript")
		}
	}
}

func TestCompressTranscriptTooShort(t *testing.T) {
	provider := &stubLLM{}
	tmpDir := t.TempDir()
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
		WithSave("compress-short-test"),
	)

	ctx := context.Background()
	// Only 1 exchange = 2 messages, at the minimum
	_, err := sess.Complete(ctx, SessionRequest{
		Prompt: Message{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	_, err = sess.CompressTranscript(ctx, 0, "Summarize.")
	if err == nil {
		t.Error("expected error for transcript too short to compress")
	}
}

func TestCompressTranscriptKeepRecentTooLarge(t *testing.T) {
	provider := &stubLLM{}
	tmpDir := t.TempDir()
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
		WithSave("compress-keep-large-test"),
	)

	ctx := context.Background()
	for i := range 5 {
		_, err := sess.Complete(ctx, SessionRequest{
			Prompt: Message{Role: RoleUser, Content: fmt.Sprintf("msg %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	// keep_recent >= total messages should fail
	_, err := sess.CompressTranscript(ctx, 10, "Summarize.")
	if err == nil {
		t.Error("expected error when keep_recent >= transcript size")
	}
}
