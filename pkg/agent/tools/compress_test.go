package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

func TestCompressToolName(t *testing.T) {
	provider := &stubLLM{}
	sess := session.NewSession(provider)
	tool := NewCompressTool(sess)
	if tool.Name() != "compress_transcript" {
		t.Errorf("expected name 'compress_transcript', got %q", tool.Name())
	}
}

func TestCompressToolEnabled(t *testing.T) {
	// Nil session should be disabled
	tool := NewCompressTool(nil)
	if tool.Enabled() {
		t.Error("expected tool to be disabled with nil session")
	}

	// Empty session should be disabled
	provider := &stubLLM{}
	sess := session.NewSession(provider)
	tool2 := NewCompressTool(sess)
	if tool2.Enabled() {
		t.Error("expected tool to be disabled with empty session")
	}

	// Session with enough messages should be enabled
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	sess3 := session.NewSession(provider, session.WithPersistenceManager(pm), session.WithSave("compress-enabled-test"))
	ctx := context.Background()
	// Add enough messages to exceed the compressToolThreshold (10)
	for i := range 6 {
		_, err := sess3.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}
	tool3 := NewCompressTool(sess3)
	// 6 exchanges = 12 messages, which exceeds threshold of 10
	if !tool3.Enabled() {
		t.Errorf("expected tool to be enabled with %d messages, threshold is %d", sess3.Size(), compressToolThreshold)
	}
}

func TestCompressToolDefaultParameters(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	sess := session.NewSession(provider,
		session.WithPersistenceManager(pm),
		session.WithSave("compress-defaults"),
	)

	ctx := context.Background()
	// Add enough messages for compression
	for i := range 10 {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	tool := NewCompressTool(sess)
	beforeSize := sess.Size()

	// Execute with empty params (all defaults)
	input, _ := json.Marshal(map[string]any{})
	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute with defaults failed: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
	if int(res["messages_before"].(float64)) != beforeSize {
		t.Errorf("expected messages_before=%d, got %v", beforeSize, res["messages_before"])
	}
	// After compression, should have fewer messages
	afterSize := int(res["messages_after"].(float64))
	if afterSize >= beforeSize {
		t.Errorf("expected fewer messages after compression, got before=%d after=%d", beforeSize, afterSize)
	}
}

func TestCompressToolCustomKeepRecent(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	sess := session.NewSession(provider,
		session.WithPersistenceManager(pm),
		session.WithSave("compress-custom"),
	)

	ctx := context.Background()
	for i := range 10 {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}

	tool := NewCompressTool(sess)

	// Execute with custom keep_recent
	input, _ := json.Marshal(map[string]any{"keep_recent": 4})
	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute with custom keep_recent failed: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Should have 1 summary + 4 recent = 5 messages
	if afterSize := int(res["messages_after"].(float64)); afterSize != 5 {
		t.Errorf("expected 5 messages after compression with keep_recent=4, got %d", afterSize)
	}
}

func TestCompressToolParametersOptional(t *testing.T) {
	tool := NewCompressTool(nil)
	params := tool.Parameters()

	for _, p := range params {
		if p.Required {
			t.Errorf("parameter %q should be optional, got required=true", p.Name)
		}
	}
}
