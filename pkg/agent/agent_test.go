package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

// mockLLM simulates an LLM that responds with predefined responses.
type mockLLM struct {
	responses []string
	callCount int
}

func (m *mockLLM) Complete(ctx context.Context, req llm.ChatRequest) (llm.Message, error) {
	if m.callCount >= len(m.responses) {
		return llm.Message{
			Role:      llm.RoleAssistant,
			Content:   "No more responses",
			Timestamp: time.Now(),
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++

	return llm.Message{
		Role:      llm.RoleAssistant,
		Content:   response,
		Timestamp: time.Now(),
	}, nil
}

// mockTool is a simple test tool.
type mockTool struct {
	name        string
	description string
	executed    bool
	response    string
	err         error
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	m.executed = true
	return m.response, m.err
}

func TestNewAgent(t *testing.T) {
	t.Run("requires session", func(t *testing.T) {
		_, err := NewAgent(Config{})
		if err == nil {
			t.Fatal("expected error when session is nil")
		}
		if !strings.Contains(err.Error(), "session is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("creates with default memory", func(t *testing.T) {
		mockLLM := &mockLLM{responses: []string{"hello"}}
		session := llm.NewSession(mockLLM, llm.SessionConfig{})

		agent, err := NewAgent(Config{Session: session})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.memory == nil {
			t.Fatal("memory should be initialized")
		}
	})

	t.Run("uses provided memory", func(t *testing.T) {
		mockLLM := &mockLLM{responses: []string{"hello"}}
		session := llm.NewSession(mockLLM, llm.SessionConfig{})
		customMemory := NewInMemoryStore()

		agent, err := NewAgent(Config{
			Session: session,
			Memory:  customMemory,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.memory != customMemory {
			t.Fatal("agent should use provided memory")
		}
	})

	t.Run("registers tools", func(t *testing.T) {
		mockLLM := &mockLLM{responses: []string{"hello"}}
		session := llm.NewSession(mockLLM, llm.SessionConfig{})
		tool := &mockTool{name: "test_tool", description: "test"}

		agent, err := NewAgent(Config{
			Session: session,
			Tools:   []Tool{tool},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agent.tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(agent.tools))
		}
		if agent.tools["test_tool"] != tool {
			t.Fatal("tool not registered correctly")
		}
	})
}

func TestAgentExecuteWithoutTools(t *testing.T) {
	mockLLM := &mockLLM{
		responses: []string{"I can help you with that!"},
	}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	response, err := agent.Execute(ctx, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response != "I can help you with that!" {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestAgentExecuteWithToolCall(t *testing.T) {
	tool := &mockTool{
		name:        "search",
		description: "search for something",
		response:    `{"results": ["item1", "item2"]}`,
	}

	mockLLM := &mockLLM{
		responses: []string{
			// First response: make a tool call
			`I'll search for that. <tool_call>{"tool_name": "search", "input": {"query": "test"}}</tool_call>`,
			// Second response: process results
			"Based on the search results, I found item1 and item2.",
		},
	}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{
		Session: session,
		Tools:   []Tool{tool},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	response, err := agent.Execute(ctx, "search for test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !tool.executed {
		t.Fatal("tool should have been executed")
	}

	if !strings.Contains(response, "item1") {
		t.Fatalf("response should mention search results: %s", response)
	}
}

func TestAgentExecuteWithMemory(t *testing.T) {
	mockLLM := &mockLLM{
		responses: []string{"Based on your preference for Delta, I recommend that airline."},
	}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	memory := NewInMemoryStore()
	memory.Store("preferred_airline", "Delta Airlines")

	agent, err := NewAgent(Config{
		Session: session,
		Memory:  memory,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	response, err := agent.Execute(ctx, "book a flight with my preferred airline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(response, "Delta") {
		t.Fatalf("response should use memory context: %s", response)
	}
}

func TestParseToolCalls(t *testing.T) {
	agent := &Agent{}

	tests := []struct {
		name     string
		response string
		expected int
	}{
		{
			name:     "no tool calls",
			response: "Just a regular response",
			expected: 0,
		},
		{
			name:     "single tool call",
			response: `Let me search. <tool_call>{"tool_name": "search", "input": {"q": "test"}}</tool_call>`,
			expected: 1,
		},
		{
			name: "multiple tool calls",
			response: `First search: <tool_call>{"tool_name": "search", "input": {"q": "test1"}}</tool_call>
			Then another: <tool_call>{"tool_name": "search", "input": {"q": "test2"}}</tool_call>`,
			expected: 2,
		},
		{
			name:     "malformed tool call",
			response: `<tool_call>not valid json</tool_call>`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := agent.parseToolCalls(tt.response)
			if len(calls) != tt.expected {
				t.Fatalf("expected %d tool calls, got %d", tt.expected, len(calls))
			}
		})
	}
}

func TestAgentMemoryOperations(t *testing.T) {
	mockLLM := &mockLLM{responses: []string{"ok"}}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Store facts
	agent.Memory().Store("key1", "value1")
	agent.Memory().Store("key2", "value2")

	// Retrieve fact
	val := agent.Memory().Retrieve("key1")
	if val != "value1" {
		t.Fatalf("expected 'value1', got %s", val)
	}

	// Search facts
	results := agent.Memory().Search("value")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Get all facts
	all := agent.Memory().All()
	if len(all) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(all))
	}

	// Clear memory
	agent.Memory().Clear()
	all = agent.Memory().All()
	if len(all) != 0 {
		t.Fatalf("expected 0 facts after clear, got %d", len(all))
	}
}

func TestAgentRegisterTool(t *testing.T) {
	mockLLM := &mockLLM{responses: []string{"ok"}}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := &mockTool{name: "new_tool", description: "test"}
	agent.RegisterTool(tool)

	if agent.tools["new_tool"] != tool {
		t.Fatal("tool not registered")
	}
}

func TestFlightTools(t *testing.T) {
	ctx := context.Background()

	t.Run("flight search tool", func(t *testing.T) {
		tool := NewFlightSearchTool()

		if tool.Name() != "flight_search" {
			t.Fatalf("unexpected tool name: %s", tool.Name())
		}

		input := json.RawMessage(`{"from": "New York", "to": "London", "date": "2026-03-15"}`)
		output, err := tool.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(output, "AA101") {
			t.Fatalf("output should contain flight numbers: %s", output)
		}
	})

	t.Run("flight reservation tool", func(t *testing.T) {
		tool := NewFlightReservationTool()

		if tool.Name() != "flight_reservation" {
			t.Fatalf("unexpected tool name: %s", tool.Name())
		}

		input := json.RawMessage(`{"flight_number": "AA101", "passenger_name": "John Doe"}`)
		output, err := tool.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(output, "CONF-") {
			t.Fatalf("output should contain confirmation number: %s", output)
		}
	})

	t.Run("flight reservation failure", func(t *testing.T) {
		tool := NewFlightReservationTool()

		input := json.RawMessage(`{"flight_number": "XX999", "passenger_name": "John Doe"}`)
		_, err := tool.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for fully booked flight")
		}
		if !strings.Contains(err.Error(), "fully booked") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
