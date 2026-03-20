package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

func TestFormatToolParametersEmpty(t *testing.T) {
	result := FormatToolParameters(nil)
	if result != "" {
		t.Errorf("expected empty string for nil params, got %q", result)
	}
	result = FormatToolParameters([]ToolParameter{})
	if result != "" {
		t.Errorf("expected empty string for empty params, got %q", result)
	}
}

func TestFormatToolParametersXML(t *testing.T) {
	params := []ToolParameter{
		{Name: "command", Type: "string", Required: true, Description: "The shell command to execute."},
		{Name: "timeout", Type: "int", Required: false, Description: "Timeout in seconds."},
	}
	result := FormatToolParameters(params)

	// Should use XML <parameter> elements
	if !strings.Contains(result, "<parameter") {
		t.Error("expected XML <parameter> element in output")
	}
	if !strings.Contains(result, "</parameter>") {
		t.Error("expected closing </parameter> tag in output")
	}

	// Verify required parameter attributes
	if !strings.Contains(result, `name="command"`) {
		t.Error("expected name attribute for command parameter")
	}
	if !strings.Contains(result, `type="string"`) {
		t.Error("expected type attribute for command parameter")
	}
	if !strings.Contains(result, `required="true"`) {
		t.Error("expected required=true for command parameter")
	}

	// Verify optional parameter attributes
	if !strings.Contains(result, `name="timeout"`) {
		t.Error("expected name attribute for timeout parameter")
	}
	if !strings.Contains(result, `required="false"`) {
		t.Error("expected required=false for timeout parameter")
	}

	// Verify descriptions are included
	if !strings.Contains(result, "The shell command to execute.") {
		t.Error("expected description for command parameter")
	}
	if !strings.Contains(result, "Timeout in seconds.") {
		t.Error("expected description for timeout parameter")
	}
}

func TestBuildSystemPromptWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an AGENTS.md file in the workspace
	agentsContent := "# Test Agent Config"
	if err := os.WriteFile(tmpDir+"/AGENTS.md", []byte(agentsContent), 0644); err != nil {
		t.Fatalf("Failed to write AGENTS.md: %v", err)
	}

	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	sess := session.NewSession(&stubLLM{})
	ag, err := NewAgent(sess,
		WithPersistenceManager(pm),
		WithSkills(skills.NewSkills()),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	prompt := ag.BuildSystemPrompt()

	// Verify XML-structured context section
	if !strings.Contains(prompt, "<context>") {
		t.Error("expected <context> section in system prompt")
	}
	if !strings.Contains(prompt, "</context>") {
		t.Error("expected </context> closing tag in system prompt")
	}
	if !strings.Contains(prompt, "Current time:") {
		t.Error("expected current time in system prompt")
	}
	if !strings.Contains(prompt, "Workspace:") {
		t.Error("expected workspace path in system prompt")
	}

	// Verify workspace files section
	if !strings.Contains(prompt, "<workspace_files>") {
		t.Error("expected <workspace_files> section in system prompt")
	}
	if !strings.Contains(prompt, "</workspace_files>") {
		t.Error("expected </workspace_files> closing tag in system prompt")
	}

	// Verify AGENTS.md content is included
	if !strings.Contains(prompt, agentsContent) {
		t.Error("expected AGENTS.md content in system prompt")
	}
}

func TestBuildSystemPromptWithBootstrap(t *testing.T) {
	tmpDir := t.TempDir()

	// Create AGENTS.md and BOOTSTRAP.md
	if err := os.WriteFile(tmpDir+"/AGENTS.md", []byte("# Agents"), 0644); err != nil {
		t.Fatalf("Failed to write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(tmpDir+"/BOOTSTRAP.md", []byte("# Bootstrap"), 0644); err != nil {
		t.Fatalf("Failed to write BOOTSTRAP.md: %v", err)
	}

	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	sess := session.NewSession(&stubLLM{})
	ag, err := NewAgent(sess,
		WithPersistenceManager(pm),
		WithSkills(skills.NewSkills()),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	prompt := ag.BuildSystemPrompt()

	// Verify bootstrap section is present with XML tags
	if !strings.Contains(prompt, "<bootstrap>") {
		t.Error("expected <bootstrap> section when BOOTSTRAP.md exists")
	}
	if !strings.Contains(prompt, "</bootstrap>") {
		t.Error("expected </bootstrap> closing tag")
	}
}

func TestBuildSystemPromptWithTools(t *testing.T) {
	sess := session.NewSession(&stubLLM{})
	ag, err := NewAgent(sess,
		WithTools(&stubTool{
			name:        "test_tool",
			description: "A test tool",
			enabled:     true,
			params: []ToolParameter{
				{Name: "input", Type: "string", Required: true, Description: "The input value."},
			},
		}),
		WithSkills(skills.NewSkills()),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	prompt := ag.BuildSystemPrompt()

	// Verify XML tool structure
	if !strings.Contains(prompt, "<tools>") {
		t.Error("expected <tools> section in system prompt")
	}
	if !strings.Contains(prompt, "</tools>") {
		t.Error("expected </tools> closing tag in system prompt")
	}
	if !strings.Contains(prompt, `<tool name="test_tool"`) {
		t.Error("expected <tool> element with name attribute")
	}
	if !strings.Contains(prompt, `description="A test tool"`) {
		t.Error("expected description attribute on tool element")
	}
	if !strings.Contains(prompt, "</tool>") {
		t.Error("expected </tool> closing tag")
	}

	// Verify XML parameter structure within tool
	if !strings.Contains(prompt, `<parameter name="input"`) {
		t.Error("expected <parameter> element within tool")
	}

	// Verify instructions section
	if !strings.Contains(prompt, "<instructions>") {
		t.Error("expected <instructions> section in system prompt")
	}
	if !strings.Contains(prompt, "</instructions>") {
		t.Error("expected </instructions> closing tag in system prompt")
	}
	if !strings.Contains(prompt, "<tool_call") {
		t.Error("expected tool_call example in instructions")
	}
}

func TestBuildSystemPromptNoXMLWithoutPersistence(t *testing.T) {
	sess := session.NewSession(&stubLLM{})
	ag, err := NewAgent(sess,
		WithSkills(skills.NewSkills()),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	prompt := ag.BuildSystemPrompt()

	// Without persistence manager or tools, prompt should be empty
	if prompt != "" {
		t.Errorf("expected empty prompt without persistence or tools, got %q", prompt)
	}
}

func TestBuildSystemPromptSubSessionHint(t *testing.T) {
	sess := session.NewSession(&stubLLM{})
	ag, err := NewAgent(sess,
		WithTools(
			&stubTool{name: "bash", description: "Run command", enabled: true},
			&stubTool{name: "sub_session", description: "Delegate tasks", enabled: true},
		),
		WithSkills(skills.NewSkills()),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	prompt := ag.BuildSystemPrompt()

	if !strings.Contains(prompt, "sub_session") {
		t.Error("expected sub_session hint when sub_session tool is present")
	}
}

// stubLLM is a minimal LLM implementation for testing.
type stubLLM struct{}

func (s *stubLLM) Complete(_ context.Context, req llm.ChatRequest) (llm.Message, error) {
	return llm.Message{Content: "stub response"}, nil
}

// stubTool is a minimal Tool implementation for testing.
type stubTool struct {
	name        string
	description string
	enabled     bool
	params      []ToolParameter
}

func (s *stubTool) Name() string                  { return s.name }
func (s *stubTool) Description() string            { return s.description }
func (s *stubTool) Parameters() []ToolParameter    { return s.params }
func (s *stubTool) Enabled() bool                  { return s.enabled }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true}`), nil
}
