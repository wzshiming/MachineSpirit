package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
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

func TestSessionSaveAndLoad(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a persistence manager
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Create a session with some data
	provider := &stubLLM{}
	seedTranscript := []llm.Message{
		{Role: llm.RoleAssistant, Content: "seed message", Timestamp: time.Unix(1, 0)},
	}
	session1 := NewSession(provider, WithTranscript(seedTranscript), WithPersistenceManager(pm))

	// Add some conversation
	ctx := context.Background()
	_, err = session1.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	// Save the session
	err = session1.Save("test-session")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Verify the file was created
	sessionFile := filepath.Join(tmpDir, "session", "test-session.ndjson")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Fatalf("Session file was not created: %s", sessionFile)
	}

	// Create a new session and load the data
	session2 := NewSession(provider, WithPersistenceManager(pm))
	err = session2.Load("test-session")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Verify the loaded session has the same data
	if len(session2.Transcript()) != len(session1.Transcript()) {
		t.Fatalf("Loaded transcript length mismatch: expected %d, got %d",
			len(session1.Transcript()), len(session2.Transcript()))
	}

	transcript1 := session1.Transcript()
	transcript2 := session2.Transcript()
	for i := range transcript1 {
		if transcript1[i].Role != transcript2[i].Role {
			t.Errorf("Message %d role mismatch: expected %s, got %s",
				i, transcript1[i].Role, transcript2[i].Role)
		}
		if transcript1[i].Content != transcript2[i].Content {
			t.Errorf("Message %d content mismatch: expected %q, got %q",
				i, transcript1[i].Content, transcript2[i].Content)
		}
	}
}

func TestSessionSaveWithoutPersistenceManager(t *testing.T) {
	provider := &stubLLM{}
	session := NewSession(provider)

	err := session.Save("test")
	if err == nil {
		t.Fatal("Expected error when saving without persistence manager")
	}
}

func TestSessionLoadNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	session := NewSession(provider, WithPersistenceManager(pm))

	err = session.Load("nonexistent")
	if err == nil {
		t.Fatal("Expected error when loading nonexistent file")
	}
}

func TestSessionSaveAddsNDJSONExtension(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	session := NewSession(provider, WithPersistenceManager(pm))

	// Save without .ndjson extension
	err = session.Save("test-session")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Verify the file was created with .ndjson extension
	sessionFile := filepath.Join(tmpDir, "session", "test-session.ndjson")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Fatalf("Session file was not created with .ndjson extension: %s", sessionFile)
	}

	// Load without .ndjson extension should also work
	session2 := NewSession(provider, WithPersistenceManager(pm))
	err = session2.Load("test-session")
	if err != nil {
		t.Fatalf("Load without .ndjson extension failed: %v", err)
	}
}

func TestAutoSaveSession(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	session := NewSession(provider,
		WithPersistenceManager(pm),
		WithAutoSave("auto-test"),
	)

	// Complete a conversation - should auto-save
	ctx := context.Background()
	_, err = session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "test message"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	// Verify the session was auto-saved
	sessionFile := filepath.Join(tmpDir, "session", "auto-test.ndjson")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Fatalf("Session was not auto-saved: %s", sessionFile)
	}

	// Load the auto-saved session
	session2 := NewSession(provider, WithPersistenceManager(pm))
	err = session2.Load("auto-test")
	if err != nil {
		t.Fatalf("Failed to load auto-saved session: %v", err)
	}

	if len(session2.Transcript()) != 2 {
		t.Fatalf("Expected 2 messages in loaded session, got %d", len(session2.Transcript()))
	}
}

func TestAutoSaveWithCompression(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	session := NewSession(provider,
		WithPersistenceManager(pm),
		WithAutoSave("compress-test"),
	)

	// Add several messages
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, err = session.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	// Compress the transcript
	err = session.CompressTranscript(ctx, 4, "Summarize the following conversation:")
	if err != nil {
		t.Fatalf("CompressTranscript returned error: %v", err)
	}

	// Verify the compressed session was auto-saved
	sessionFile := filepath.Join(tmpDir, "session", "compress-test.ndjson")
	content, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read auto-saved session: %v", err)
	}

	// Verify it contains message data in JSONL format
	// Each line should be a JSON object
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 {
		t.Fatalf("Auto-saved session is empty")
	}

	// Load and verify
	session2 := NewSession(provider, WithPersistenceManager(pm))
	err = session2.Load("compress-test")
	if err != nil {
		t.Fatalf("Failed to load compressed session: %v", err)
	}

	// Should have base + summary + 4 recent messages = 5 messages
	if session2.Size() != 5 {
		t.Fatalf("Expected 5 messages after compression, got %d", session2.Size())
	}
}

func TestSessionAppendBehavior(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	session := NewSession(provider, WithPersistenceManager(pm))

	ctx := context.Background()

	// Add first message and save
	_, err = session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "first message"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	err = session.Save("append-test")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Read the file to check it has 2 lines (user + assistant)
	sessionFile := filepath.Join(tmpDir, "session", "append-test.ndjson")
	content1, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file: %v", err)
	}
	lines1 := strings.Split(strings.TrimSpace(string(content1)), "\n")
	if len(lines1) != 2 {
		t.Fatalf("Expected 2 lines after first save, got %d", len(lines1))
	}

	// Add second message and save again
	_, err = session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "second message"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	err = session.Save("append-test")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Read the file again - should now have 4 lines (2 exchanges)
	content2, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file after second save: %v", err)
	}
	lines2 := strings.Split(strings.TrimSpace(string(content2)), "\n")
	if len(lines2) != 4 {
		t.Fatalf("Expected 4 lines after second save (append), got %d", len(lines2))
	}

	// Verify the first two lines are unchanged
	if lines2[0] != lines1[0] || lines2[1] != lines1[1] {
		t.Fatalf("First two lines changed after append - file was rewritten instead of appended")
	}

	// Load the session in a new instance and verify all messages are present
	session2 := NewSession(provider, WithPersistenceManager(pm))
	err = session2.Load("append-test")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if session2.Size() != 4 {
		t.Fatalf("Expected 4 messages after load, got %d", session2.Size())
	}
}

func TestCompressionDoesNotForceResetSavedCount(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	session := NewSession(provider,
		WithPersistenceManager(pm),
		WithAutoSave("compression-append-test"),
	)

	ctx := context.Background()

	// Add 10 messages to build up the transcript
	for i := 0; i < 10; i++ {
		_, err = session.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	// Get file content before compression
	sessionFile := filepath.Join(tmpDir, "session", "compression-append-test.ndjson")
	contentBeforeCompression, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file before compression: %v", err)
	}
	linesBeforeCompression := strings.Split(strings.TrimSpace(string(contentBeforeCompression)), "\n")

	// Compress the transcript, keeping 4 recent messages
	err = session.CompressTranscript(ctx, 4, "Summarize the following conversation:")
	if err != nil {
		t.Fatalf("CompressTranscript returned error: %v", err)
	}

	// After compression, the file should be rewritten (smaller)
	contentAfterCompression, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file after compression: %v", err)
	}
	linesAfterCompression := strings.Split(strings.TrimSpace(string(contentAfterCompression)), "\n")

	// Should have summary + 4 recent messages = 5 messages
	if len(linesAfterCompression) != 5 {
		t.Fatalf("Expected 5 lines after compression, got %d", len(linesAfterCompression))
	}

	if len(linesAfterCompression) >= len(linesBeforeCompression) {
		t.Fatalf("File should be smaller after compression: before=%d, after=%d",
			len(linesBeforeCompression), len(linesAfterCompression))
	}

	// Now add one more message - this should APPEND, not rewrite
	_, err = session.Complete(ctx, llm.ChatRequest{
		Prompt: llm.Message{Role: llm.RoleUser, Content: "post-compression message"},
	})
	if err != nil {
		t.Fatalf("Complete after compression returned error: %v", err)
	}

	// Read the file again
	contentAfterNewMessage, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file after new message: %v", err)
	}
	linesAfterNewMessage := strings.Split(strings.TrimSpace(string(contentAfterNewMessage)), "\n")

	// Should now have 7 lines (5 from compression + 2 new messages)
	if len(linesAfterNewMessage) != 7 {
		t.Fatalf("Expected 7 lines after new message, got %d", len(linesAfterNewMessage))
	}

	// Verify that the first 5 lines are unchanged (append, not rewrite)
	for i := 0; i < 5; i++ {
		if linesAfterNewMessage[i] != linesAfterCompression[i] {
			t.Fatalf("Line %d changed after new message - file was rewritten instead of appended", i)
		}
	}

	// Verify we can still load the session correctly
	session2 := NewSession(provider, WithPersistenceManager(pm))
	err = session2.Load("compression-append-test")
	if err != nil {
		t.Fatalf("Failed to load session after compression and new message: %v", err)
	}

	if session2.Size() != 7 {
		t.Fatalf("Expected 7 messages after load, got %d", session2.Size())
	}
}
