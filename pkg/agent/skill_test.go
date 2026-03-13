package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

// mockSkill is a simple test skill.
type mockSkill struct {
	name        string
	description string
	executed    bool
	response    string
	err         error
}

func (m *mockSkill) Name() string {
	return m.name
}

func (m *mockSkill) Description() string {
	return m.description
}

func (m *mockSkill) DetailedDescription() string {
	return m.description + " (detailed)"
}

func (m *mockSkill) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

func (m *mockSkill) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	m.executed = true
	return m.response, m.err
}

func TestSkillRegistry(t *testing.T) {
	registry := NewSkillRegistry()

	skill := &mockSkill{name: "test_skill", description: "test"}

	t.Run("register skill", func(t *testing.T) {
		err := registry.Register(skill)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if registry.Count() != 1 {
			t.Fatalf("expected 1 skill, got %d", registry.Count())
		}
	})

	t.Run("get registered skill", func(t *testing.T) {
		retrieved, err := registry.Get("test_skill")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if retrieved != skill {
			t.Fatal("retrieved skill does not match")
		}
	})

	t.Run("has skill", func(t *testing.T) {
		if !registry.Has("test_skill") {
			t.Fatal("registry should have test_skill")
		}
		if registry.Has("nonexistent") {
			t.Fatal("registry should not have nonexistent")
		}
	})

	t.Run("list skills", func(t *testing.T) {
		skills := registry.List()
		if len(skills) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(skills))
		}
	})

	t.Run("duplicate registration fails", func(t *testing.T) {
		err := registry.Register(skill)
		if err == nil {
			t.Fatal("expected error for duplicate registration")
		}
	})

	t.Run("unregister skill", func(t *testing.T) {
		err := registry.Unregister("test_skill")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if registry.Count() != 0 {
			t.Fatalf("expected 0 skills after unregister, got %d", registry.Count())
		}
	})

	t.Run("get nonexistent skill", func(t *testing.T) {
		_, err := registry.Get("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent skill")
		}
	})
}

func TestSkillInvoker(t *testing.T) {
	registry := NewSkillRegistry()
	skill := &mockSkill{
		name:        "test_skill",
		description: "test",
		response:    "skill result",
	}
	registry.Register(skill)

	invoker := NewSkillInvoker(registry)

	t.Run("invoke skill", func(t *testing.T) {
		ctx := context.Background()
		input := json.RawMessage(`{"query": "test"}`)
		result, err := invoker.Invoke(ctx, "test_skill", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "skill result" {
			t.Fatalf("unexpected result: %s", result)
		}
		if !skill.executed {
			t.Fatal("skill should have been executed")
		}
	})

	t.Run("has skill", func(t *testing.T) {
		if !invoker.Has("test_skill") {
			t.Fatal("invoker should have test_skill")
		}
	})

	t.Run("list skills", func(t *testing.T) {
		names := invoker.List()
		if len(names) != 1 || names[0] != "test_skill" {
			t.Fatalf("unexpected list: %v", names)
		}
	})
}

func TestMultiToolInvoker(t *testing.T) {
	// Setup tools
	tool := &mockTool{
		name:        "test_tool",
		description: "test tool",
		response:    "tool result",
	}
	tools := map[string]Tool{"test_tool": tool}

	// Setup skills
	registry := NewSkillRegistry()
	skill := &mockSkill{
		name:        "test_skill",
		description: "test skill",
		response:    "skill result",
	}
	registry.Register(skill)

	invoker := NewMultiToolInvoker(tools, registry)
	ctx := context.Background()

	t.Run("invoke tool explicitly", func(t *testing.T) {
		input := json.RawMessage(`{"query": "test"}`)
		result, err := invoker.Invoke(ctx, ToolKindTool, "test_tool", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "tool result" {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("invoke skill explicitly", func(t *testing.T) {
		input := json.RawMessage(`{"query": "test"}`)
		result, err := invoker.Invoke(ctx, ToolKindSkill, "test_skill", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "skill result" {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("invoke auto detects skill", func(t *testing.T) {
		input := json.RawMessage(`{"query": "test"}`)
		kind, result, err := invoker.InvokeAuto(ctx, "test_skill", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != ToolKindSkill {
			t.Fatalf("expected skill kind, got %s", kind)
		}
		if result != "skill result" {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("invoke auto detects tool", func(t *testing.T) {
		input := json.RawMessage(`{"query": "test"}`)
		kind, result, err := invoker.InvokeAuto(ctx, "test_tool", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != ToolKindTool {
			t.Fatalf("expected tool kind, got %s", kind)
		}
		if result != "tool result" {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("list all", func(t *testing.T) {
		all := invoker.ListAll()
		if len(all[ToolKindTool]) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(all[ToolKindTool]))
		}
		if len(all[ToolKindSkill]) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(all[ToolKindSkill]))
		}
	})
}

func TestAgentWithSkills(t *testing.T) {
	mockLLM := &mockLLM{
		responses: []string{"I'll use the skill. <tool_call>{\"tool_name\": \"test_skill\", \"input\": {\"query\": \"test\"}}</tool_call>",
			"Based on the skill results, the answer is ready."},
	}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	registry := NewSkillRegistry()
	skill := &mockSkill{
		name:        "test_skill",
		description: "test skill",
		response:    "skill executed successfully",
	}
	registry.Register(skill)

	agent, err := NewAgent(Config{
		Session: session,
		Skills:  registry,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	response, err := agent.Execute(ctx, "test request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !skill.executed {
		t.Fatal("skill should have been executed")
	}

	if !strings.Contains(response, "answer is ready") {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestAgentRegisterSkill(t *testing.T) {
	mockLLM := &mockLLM{responses: []string{"ok"}}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skill := &mockSkill{name: "new_skill", description: "test"}
	err = agent.RegisterSkill(skill)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !agent.GetSkillRegistry().Has("new_skill") {
		t.Fatal("skill not registered")
	}
}

func TestFlightBookingSkill(t *testing.T) {
	ctx := context.Background()

	searchTool := NewFlightSearchTool()
	reservationTool := NewFlightReservationTool()

	skill := NewFlightBookingSkill(searchTool, reservationTool)

	t.Run("name and description", func(t *testing.T) {
		if skill.Name() != "flight_booking" {
			t.Fatalf("unexpected name: %s", skill.Name())
		}
		if skill.Description() == "" {
			t.Fatal("description should not be empty")
		}
		if skill.DetailedDescription() == "" {
			t.Fatal("detailed description should not be empty")
		}
	})

	t.Run("execute successfully", func(t *testing.T) {
		input := json.RawMessage(`{
			"from": "New York",
			"to": "London",
			"date": "2026-03-15",
			"passenger_name": "John Doe"
		}`)

		output, err := skill.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(output, "success") {
			t.Fatalf("output should indicate success: %s", output)
		}
	})

	t.Run("execute with preferred airline", func(t *testing.T) {
		input := json.RawMessage(`{
			"from": "New York",
			"to": "London",
			"date": "2026-03-15",
			"passenger_name": "Jane Doe",
			"preferred_airline": "Delta Airlines"
		}`)

		output, err := skill.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that Delta was selected
		if !strings.Contains(output, "DL303") {
			t.Fatalf("should select Delta flight: %s", output)
		}
	})
}

func TestToolAsSkill(t *testing.T) {
	tool := NewFlightSearchTool()
	skill := NewToolAsSkill(tool)

	if skill.Name() != tool.Name() {
		t.Fatal("wrapped skill should have same name as tool")
	}
	if skill.Description() != tool.Description() {
		t.Fatal("wrapped skill should have same description as tool")
	}

	ctx := context.Background()
	input := json.RawMessage(`{"from": "NYC", "to": "LON", "date": "2026-03-15"}`)

	toolOutput, toolErr := tool.Execute(ctx, input)
	skillOutput, skillErr := skill.Execute(ctx, input)

	if toolErr != skillErr {
		t.Fatal("tool and skill should return same error")
	}
	if toolOutput != skillOutput {
		t.Fatal("tool and skill should return same output")
	}
}
