package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
)

// mockLLM is a test double that returns predefined responses.
type mockLLM struct {
	responses []string
	callCount int
}

func (m *mockLLM) Complete(ctx context.Context, req llm.ChatRequest) (llm.Message, error) {
	if m.callCount >= len(m.responses) {
		return llm.Message{}, errors.New("no more mock responses available")
	}
	response := m.responses[m.callCount]
	m.callCount++
	return llm.Message{
		Role:      llm.RoleAssistant,
		Content:   response,
		Timestamp: time.Now(),
	}, nil
}

// mockTool is a test tool that records executions.
type mockTool struct {
	name        string
	description string
	executions  []json.RawMessage
	response    json.RawMessage
	err         error
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	m.executions = append(m.executions, input)
	return m.response, m.err
}

func TestSubAgentToolBasicExecution(t *testing.T) {
	// Create a temporary workspace for testing
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Create mock LLM that returns a simple response (no tool calls)
	mockLLM := &mockLLM{
		responses: []string{"I have completed the analysis task. The data shows 42 records."},
	}

	// Track observations
	var observations []string
	observerFunc := func(message string) {
		observations = append(observations, message)
	}

	// Auto-approve all actions
	approvalFunc := func(ctx context.Context, action string) (bool, error) {
		return true, nil
	}

	// Create a mock tool for the subagent to use
	mockToolInstance := &mockTool{
		name:        "analyze",
		description: "Analyzes data",
		response:    json.RawMessage(`{"result": "42 records found"}`),
	}

	// Create the subagent tool
	subagentTool := NewSubAgentTool(
		mockLLM,
		pm,
		[]agent.Tool{mockToolInstance},
		approvalFunc,
		observerFunc,
	)

	// Execute the subagent
	input := SubAgentInput{
		Task:        "Analyze the data and report findings",
		Description: "Look for patterns in the dataset",
	}
	inputJSON, _ := json.Marshal(input)

	result, err := subagentTool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("SubAgentTool.Execute failed: %v", err)
	}

	// Parse the result
	var output SubAgentOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	// Verify the output
	if output.Status != "success" {
		t.Errorf("Expected status 'success', got %q", output.Status)
	}
	if output.Result == "" {
		t.Error("Expected non-empty result")
	}
	if output.Error != "" {
		t.Errorf("Unexpected error: %s", output.Error)
	}

	// Verify observations were recorded
	if len(observations) == 0 {
		t.Error("Expected observations to be recorded")
	}

	// Check that the starting observation was recorded
	foundStart := false
	for _, obs := range observations {
		if obs == "🤖 Starting subagent for task: Analyze the data and report findings" {
			foundStart = true
			break
		}
	}
	if !foundStart {
		t.Error("Expected starting observation not found")
	}
}

func TestSubAgentToolWithToolExecution(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Mock LLM that first makes a tool call, then responds
	mockLLM := &mockLLM{
		responses: []string{
			`I will use the read tool to examine the file.
<tool_call>{"tool": "read", "input": {"file": "data.txt"}}</tool_call>`,
			`Based on the file contents, I found the answer: 42`,
		},
	}

	var observations []string
	observerFunc := func(message string) {
		observations = append(observations, message)
	}

	approvalFunc := func(ctx context.Context, action string) (bool, error) {
		return true, nil
	}

	mockToolInstance := &mockTool{
		name:        "read",
		description: "Reads a file",
		response:    json.RawMessage(`{"content": "The answer is 42"}`),
	}

	subagentTool := NewSubAgentTool(
		mockLLM,
		pm,
		[]agent.Tool{mockToolInstance},
		approvalFunc,
		observerFunc,
	)

	input := SubAgentInput{
		Task: "Read data.txt and find the answer",
	}
	inputJSON, _ := json.Marshal(input)

	result, err := subagentTool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("SubAgentTool.Execute failed: %v", err)
	}

	var output SubAgentOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	if output.Status != "success" {
		t.Errorf("Expected status 'success', got %q", output.Status)
	}

	// Verify the mock tool was called
	if len(mockToolInstance.executions) != 1 {
		t.Errorf("Expected 1 tool execution, got %d", len(mockToolInstance.executions))
	}

	// Verify actions were recorded
	if len(output.Actions) != 1 {
		t.Errorf("Expected 1 action recorded, got %d", len(output.Actions))
	}

	// Check for tool-related observations
	foundToolUse := false
	for _, obs := range observations {
		if obs == "🔧 Subagent wants to use: read" {
			foundToolUse = true
			break
		}
	}
	if !foundToolUse {
		t.Error("Expected tool use observation not found")
	}
}

func TestSubAgentToolUserDeniesApproval(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	mockLLM := &mockLLM{
		responses: []string{"This should not be reached"},
	}

	// User denies approval
	approvalFunc := func(ctx context.Context, action string) (bool, error) {
		return false, nil
	}

	subagentTool := NewSubAgentTool(
		mockLLM,
		pm,
		[]agent.Tool{},
		approvalFunc,
		nil,
	)

	input := SubAgentInput{
		Task: "Do something",
	}
	inputJSON, _ := json.Marshal(input)

	result, err := subagentTool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("SubAgentTool.Execute failed: %v", err)
	}

	var output SubAgentOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	// Verify the execution was cancelled
	if output.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %q", output.Status)
	}
	if output.Cancelled == "" {
		t.Error("Expected cancelled reason to be set")
	}

	// Verify LLM was never called (callCount should be 0)
	if mockLLM.callCount != 0 {
		t.Errorf("Expected LLM not to be called, but it was called %d times", mockLLM.callCount)
	}
}

func TestSubAgentToolFilterTools(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	mockLLM := &mockLLM{
		responses: []string{"Done"},
	}

	tool1 := &mockTool{name: "read", description: "Read files"}
	tool2 := &mockTool{name: "write", description: "Write files"}
	tool3 := &mockTool{name: "bash", description: "Execute bash"}

	allTools := []agent.Tool{tool1, tool2, tool3}

	subagentTool := NewSubAgentTool(
		mockLLM,
		pm,
		allTools,
		func(ctx context.Context, action string) (bool, error) { return true, nil },
		nil,
	)

	// Test filtering specific tools
	filtered := subagentTool.filterTools([]string{"read", "bash"})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered tools, got %d", len(filtered))
	}

	// Test no filter (should return all)
	filtered = subagentTool.filterTools([]string{})
	if len(filtered) != 3 {
		t.Errorf("Expected 3 tools when no filter applied, got %d", len(filtered))
	}

	// Test with non-existent tool name
	filtered = subagentTool.filterTools([]string{"nonexistent"})
	if len(filtered) != 0 {
		t.Errorf("Expected 0 tools for non-existent tool name, got %d", len(filtered))
	}
}

func TestSubAgentToolInvalidInput(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	mockLLM := &mockLLM{
		responses: []string{"Done"},
	}

	subagentTool := NewSubAgentTool(
		mockLLM,
		pm,
		[]agent.Tool{},
		func(ctx context.Context, action string) (bool, error) { return true, nil },
		nil,
	)

	// Test with invalid JSON
	_, err = subagentTool.Execute(context.Background(), json.RawMessage(`{invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON input")
	}

	// Test with missing task
	input := SubAgentInput{
		Task: "",
	}
	inputJSON, _ := json.Marshal(input)
	_, err = subagentTool.Execute(context.Background(), inputJSON)
	if err == nil {
		t.Error("Expected error for missing task")
	}
}

func TestObservableToolWrapper(t *testing.T) {
	var observations []string
	observerFunc := func(message string) {
		observations = append(observations, message)
	}

	var actions []string
	mockToolInstance := &mockTool{
		name:        "test",
		description: "Test tool",
		response:    json.RawMessage(`{"status": "ok"}`),
	}

	observable := &observableTool{
		tool:         mockToolInstance,
		approvalFunc: func(ctx context.Context, action string) (bool, error) { return true, nil },
		observerFunc: observerFunc,
		actions:      &actions,
	}

	// Execute the tool
	input := json.RawMessage(`{"param": "value"}`)
	result, err := observable.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Observable tool execution failed: %v", err)
	}

	// Verify result is passed through
	if string(result) != `{"status": "ok"}` {
		t.Errorf("Unexpected result: %s", string(result))
	}

	// Verify observations
	if len(observations) != 3 { // wants to use, executing, completed
		t.Errorf("Expected 3 observations, got %d", len(observations))
	}

	// Verify actions recorded
	if len(actions) != 1 {
		t.Errorf("Expected 1 action recorded, got %d", len(actions))
	}

	// Verify tool was actually called
	if len(mockToolInstance.executions) != 1 {
		t.Errorf("Expected 1 tool execution, got %d", len(mockToolInstance.executions))
	}
}
