package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

func TestCompressToolName(t *testing.T) {
	provider := &stubLLM{}
	sess := session.NewSession(provider)
	addInput, _ := collectingAddInput()
	tool := NewCompressTool(sess, provider, addInput)
	if tool.Name() != "compress_transcript" {
		t.Errorf("expected name 'compress_transcript', got %q", tool.Name())
	}
}

func TestCompressToolEnabled(t *testing.T) {
	// Nil session should be disabled
	addInput, _ := collectingAddInput()
	tool := NewCompressTool(nil, nil, nil)
	if tool.Enabled() {
		t.Error("expected tool to be disabled with nil session/llm/addInput")
	}

	// Empty session should be disabled (below threshold)
	provider := &stubLLM{}
	tool2 := NewCompressTool(session.NewSession(provider), provider, addInput)
	if tool2.Enabled() {
		t.Error("expected tool to be disabled with empty session")
	}

	// Session with enough messages should be enabled
	tmpDir := t.TempDir()
	sess3 := session.NewSession(provider, session.WithBaseDir(tmpDir), session.WithSave("compress-enabled-test"))
	ctx := context.Background()
	for i := range 6 {
		_, err := sess3.Complete(ctx, session.SessionRequest{
			Prompt: session.Message{Role: session.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}
	tool3 := NewCompressTool(sess3, provider, addInput)
	// 6 exchanges = 12 messages, which exceeds threshold of 10
	if !tool3.Enabled() {
		t.Errorf("expected tool to be enabled with %d messages, threshold is %d", sess3.Size(), compressToolThreshold)
	}
}

func TestCompressToolReturnsStarted(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &stubLLM{}
	sess := session.NewSession(provider,
		session.WithBaseDir(tmpDir),
		session.WithSave("compress-started-test"),
	)

	ctx := context.Background()
	for i := range 10 {
		_, err := sess.Complete(ctx, session.SessionRequest{
			Prompt: session.Message{Role: session.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	addInput, _ := collectingAddInput()
	tool := NewCompressTool(sess, provider, addInput)

	// Execute should return immediately with "started" status
	input, _ := json.Marshal(map[string]any{"keep_recent": 4})
	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if res["status"] != "started" {
		t.Errorf("expected status 'started', got %v", res["status"])
	}
}

func TestCompressToolBackgroundCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &stubLLM{}
	sess := session.NewSession(provider,
		session.WithBaseDir(tmpDir),
		session.WithSave("compress-bg-test"),
	)

	ctx := context.Background()
	for i := range 10 {
		_, err := sess.Complete(ctx, session.SessionRequest{
			Prompt: session.Message{Role: session.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	beforeSize := sess.Size()

	addInput, getInputs := collectingAddInput()
	tool := NewCompressTool(sess, provider, addInput)

	input, _ := json.Marshal(map[string]any{"keep_recent": 4})
	_, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for the background goroutine to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(getInputs()) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify addInput was called with completion message
	msgs := getInputs()
	if len(msgs) == 0 {
		t.Fatal("expected addInput to be called after compression")
	}

	found := false
	for _, msg := range msgs {
		if msg.Role == llm.RoleUser && len(msg.Content) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected a completion message from addInput callback")
	}

	// Verify the session was actually compressed
	afterSize := sess.Size()
	if afterSize >= beforeSize {
		t.Errorf("expected fewer messages after compression, got before=%d after=%d", beforeSize, afterSize)
	}
	// 1 summary + 4 recent = 5
	if afterSize != 5 {
		t.Errorf("expected 5 messages after compression with keep_recent=4, got %d", afterSize)
	}
}

func TestCompressToolParametersOptional(t *testing.T) {
	tool := NewCompressTool(nil, nil, nil)
	params := tool.Parameters()

	for _, p := range params {
		if p.Required {
			t.Errorf("parameter %q should be optional, got required=true", p.Name)
		}
	}
}
