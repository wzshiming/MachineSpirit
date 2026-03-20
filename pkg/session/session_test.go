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

func TestPrepareCompressValidation(t *testing.T) {
	provider := &stubLLM{}
	tmpDir := t.TempDir()
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
		WithSave("prepare-compress-test"),
	)

	// Empty transcript should fail
	_, _, _, err := sess.PrepareCompress(4)
	if err == nil {
		t.Error("expected error for empty transcript")
	}

	ctx := context.Background()
	// Add 2 messages (1 exchange) - at minimum, should fail
	_, err = sess.Complete(ctx, SessionRequest{
		Prompt: Message{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	_, _, _, err = sess.PrepareCompress(0)
	if err == nil {
		t.Error("expected error for transcript too short")
	}

	// Add more messages
	for i := range 9 {
		_, err := sess.Complete(ctx, SessionRequest{
			Prompt: Message{Role: RoleUser, Content: fmt.Sprintf("msg %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	// keep_recent >= transcript size should fail
	_, _, _, err = sess.PrepareCompress(20)
	if err == nil {
		t.Error("expected error when keep_recent >= transcript size")
	}

	// Valid call should succeed
	text, keep, origSize, err := sess.PrepareCompress(4)
	if err != nil {
		t.Fatalf("PrepareCompress failed: %v", err)
	}
	if keep != 4 {
		t.Errorf("expected keep=4, got %d", keep)
	}
	if origSize != 20 {
		t.Errorf("expected originalSize=20, got %d", origSize)
	}
	if text == "" {
		t.Error("expected non-empty text to summarize")
	}
}

func TestApplyCompression(t *testing.T) {
	provider := &stubLLM{}
	tmpDir := t.TempDir()
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
		WithSave("apply-compress-test"),
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

	if sess.Size() != 20 {
		t.Fatalf("expected 20 messages, got %d", sess.Size())
	}

	// Record original transcript for later verification
	transcriptBefore := sess.Transcript()

	// Apply compression: keep 4 recent, originalSize=20
	archivePath, err := sess.ApplyCompression("This is a summary.", 4, 20)
	if err != nil {
		t.Fatalf("ApplyCompression failed: %v", err)
	}
	if archivePath == "" {
		t.Error("expected non-empty archive path")
	}

	// Verify: 1 summary + 4 recent = 5 messages
	if sess.Size() != 5 {
		t.Errorf("expected 5 messages after compression, got %d", sess.Size())
	}

	transcript := sess.Transcript()

	// First message should be the summary
	if transcript[0].Role != llm.RoleAssistant {
		t.Errorf("expected first message to be assistant (summary), got %v", transcript[0].Role)
	}
	if transcript[0].Content != "This is a summary." {
		t.Errorf("expected summary content, got %q", transcript[0].Content)
	}

	// Remaining 4 messages should match the last 4 from the original
	for i := 1; i < len(transcript); i++ {
		original := transcriptBefore[len(transcriptBefore)-4+i-1]
		if transcript[i].Content != original.Content {
			t.Errorf("message %d: expected %q, got %q", i, original.Content, transcript[i].Content)
		}
	}
}

func TestApplyCompressionPreservesNewMessages(t *testing.T) {
	provider := &stubLLM{}
	tmpDir := t.TempDir()
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
		WithSave("apply-preserve-test"),
	)

	ctx := context.Background()
	// Add 20 messages
	for i := range 10 {
		_, err := sess.Complete(ctx, SessionRequest{
			Prompt: Message{Role: RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	originalSize := sess.Size() // 20

	// Simulate messages added while compression was running
	_, err := sess.Complete(ctx, SessionRequest{
		Prompt: Message{Role: RoleUser, Content: "new message during compression"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	// Now 22 messages total

	// Apply compression with originalSize=20, keepRecent=4
	_, err = sess.ApplyCompression("Summary of old messages.", 4, originalSize)
	if err != nil {
		t.Fatalf("ApplyCompression failed: %v", err)
	}

	// Should have: 1 summary + 4 recent from original + 2 new messages = 7
	if sess.Size() != 7 {
		t.Errorf("expected 7 messages after compression (preserving new messages), got %d", sess.Size())
	}

	transcript := sess.Transcript()
	// Last 2 messages should be the ones added during compression
	if transcript[len(transcript)-1].Content != "done: new message during compression" {
		t.Errorf("expected last message to be the new one added during compression, got %q", transcript[len(transcript)-1].Content)
	}
	if transcript[len(transcript)-2].Content != "new message during compression" {
		t.Errorf("expected second-to-last message to be the new one added during compression, got %q", transcript[len(transcript)-2].Content)
	}
}

func TestSessionLLMAndBaseDir(t *testing.T) {
	provider := &stubLLM{}
	tmpDir := t.TempDir()
	sess := NewSession(provider,
		WithBaseDir(tmpDir),
	)

	if sess.LLM() != provider {
		t.Error("expected LLM() to return the provider")
	}
	if sess.BaseDir() != tmpDir {
		t.Errorf("expected BaseDir()=%q, got %q", tmpDir, sess.BaseDir())
	}
}
