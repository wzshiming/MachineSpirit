package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

type mockLLM struct {
	responses []string
	callIndex int
}

func (m *mockLLM) Complete(ctx context.Context, req llm.ChatRequest) (llm.Message, error) {
	response := "default response"
	if m.callIndex < len(m.responses) {
		response = m.responses[m.callIndex]
	}
	m.callIndex++

	return llm.Message{
		Role:    llm.RoleAssistant,
		Content: response,
	}, nil
}

func TestCompressTool(t *testing.T) {
	// Create a temp directory for persistence manager
	tmpDir := t.TempDir()

	// Create MEMORY.md file (used for compression prompt)
	compressPrompt := "Summarize the conversation."
	if err := os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte(compressPrompt), 0644); err != nil {
		t.Fatalf("Failed to write MEMORY.md: %v", err)
	}

	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Create mock LLM with predefined responses
	mockLLM := &mockLLM{
		responses: []string{
			"msg1 response",
			"msg2 response",
			"msg3 response",
			"msg4 response",
			"msg5 response",
			"Summary of messages",
		},
	}

	// Create session
	sess := session.NewSession(mockLLM, session.WithPersistenceManager(pm))

	ctx := context.Background()

	// Add some messages to the transcript
	for i := 0; i < 5; i++ {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{
				Role:    llm.RoleUser,
				Content: "message",
			},
		})
		if err != nil {
			t.Fatalf("Failed to complete: %v", err)
		}
	}

	// Verify we have 10 messages (5 user + 5 assistant)
	if len(sess.Transcript()) != 10 {
		t.Fatalf("Expected 10 messages, got %d", len(sess.Transcript()))
	}

	// Create compress tool
	tool := NewCompressTool(sess)

	// Test tool name and description
	if tool.Name() != "compress_transcript" {
		t.Errorf("Expected name 'compress_transcript', got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	// Execute compression with keep_recent=4
	keepRecent := 4
	input, _ := json.Marshal(map[string]interface{}{
		"keep_recent": keepRecent,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Parse result
	var resultData map[string]interface{}
	if err := json.Unmarshal(result, &resultData); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify result
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", resultData["status"])
	}

	if resultData["messages_before"].(float64) != 10 {
		t.Errorf("Expected 10 messages before, got %v", resultData["messages_before"])
	}

	// After compression: base + 1 summary + 4 recent = 5 messages
	// (no base transcript, so just 1 summary + 4 recent = 5)
	afterCount := len(sess.Transcript())
	if afterCount != 5 {
		t.Errorf("Expected 5 messages after compression, got %d", afterCount)
	}
}

func TestCompressToolWithoutKeepRecent(t *testing.T) {
	// Create a temp directory for persistence manager
	tmpDir := t.TempDir()

	// Create MEMORY.md file (used for compression prompt)
	compressPrompt := "Summarize the conversation."
	if err := os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte(compressPrompt), 0644); err != nil {
		t.Fatalf("Failed to write MEMORY.md: %v", err)
	}

	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Create mock LLM
	mockLLM := &mockLLM{
		responses: []string{
			"msg1", "msg2", "msg3", "msg4", "msg5", "msg6", "Summary",
		},
	}

	// Create session
	sess := session.NewSession(mockLLM, session.WithPersistenceManager(pm))

	ctx := context.Background()

	// Add some messages
	for i := 0; i < 6; i++ {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{
				Role:    llm.RoleUser,
				Content: "message",
			},
		})
		if err != nil {
			t.Fatalf("Failed to complete: %v", err)
		}
	}

	// Create compress tool
	tool := NewCompressTool(sess)

	// Execute compression without keep_recent parameter (should default to half)
	input, _ := json.Marshal(map[string]interface{}{})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Parse result
	var resultData map[string]interface{}
	if err := json.Unmarshal(result, &resultData); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", resultData["status"])
	}

	// Should keep half (6) of the original messages
	afterCount := len(sess.Transcript())
	if afterCount != 7 {
		t.Errorf("Expected 7 messages after compression (1 summary + 6 recent), got %d", afterCount)
	}
}
