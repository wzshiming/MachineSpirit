package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []toolCall
	}{
		{
			name:     "no tool calls",
			input:    "Just a regular response with no tool calls.",
			expected: nil,
		},
		{
			name:  "single tool call",
			input: `Some text <tool_call name="bash">{"command": "ls"}</tool_call> more text`,
			expected: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
			},
		},
		{
			name: "multiple tool calls",
			input: `<tool_call name="bash">{"command": "ls"}</tool_call>
<tool_call name="read">{"path": "/tmp/test.txt"}</tool_call>`,
			expected: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
				{Tool: "read", Input: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
		},
		{
			name:     "missing name attribute",
			input:    `<tool_call>{"command": "ls"}</tool_call>`,
			expected: nil,
		},
		{
			name:     "missing closing tag",
			input:    `<tool_call name="bash">{"command": "ls"}`,
			expected: nil,
		},
		{
			name:  "whitespace around JSON",
			input: `<tool_call name="bash">  {"command": "ls"}  </tool_call>`,
			expected: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCalls(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d tool calls, got %d", len(tt.expected), len(result))
			}
			for i, call := range result {
				if call.Tool != tt.expected[i].Tool {
					t.Errorf("call %d: expected tool %q, got %q", i, tt.expected[i].Tool, call.Tool)
				}
				if string(call.Input) != string(tt.expected[i].Input) {
					t.Errorf("call %d: expected input %s, got %s", i, tt.expected[i].Input, call.Input)
				}
			}
		})
	}
}

func TestExtractTagAttribute(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		attr     string
		expected string
	}{
		{
			name:     "simple attribute",
			tag:      `<tool_call name="bash">`,
			attr:     "name",
			expected: "bash",
		},
		{
			name:     "attribute with underscore",
			tag:      `<tool_call name="mcp_tool">`,
			attr:     "name",
			expected: "mcp_tool",
		},
		{
			name:     "missing attribute",
			tag:      `<tool_call>`,
			attr:     "name",
			expected: "",
		},
		{
			name:     "different attribute",
			tag:      `<tool_call id="123" name="bash">`,
			attr:     "name",
			expected: "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTagAttribute(tt.tag, tt.attr)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatToolParameters(t *testing.T) {
	t.Run("empty parameters", func(t *testing.T) {
		result := FormatToolParameters(nil)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("single required parameter", func(t *testing.T) {
		params := []ToolParameter{
			{Name: "command", Type: "string", Required: true, Description: "The command to run."},
		}
		result := FormatToolParameters(params)
		if !strings.Contains(result, "Parameters:") {
			t.Error("expected 'Parameters:' header")
		}
		if !strings.Contains(result, "command (string, required): The command to run.") {
			t.Errorf("expected formatted parameter, got %q", result)
		}
	})

	t.Run("mixed required and optional", func(t *testing.T) {
		params := []ToolParameter{
			{Name: "path", Type: "string", Required: true, Description: "File path."},
			{Name: "max", Type: "int", Required: false, Description: "Max lines."},
		}
		result := FormatToolParameters(params)
		if !strings.Contains(result, "path (string, required): File path.") {
			t.Error("expected required parameter")
		}
		if !strings.Contains(result, "max (int, optional): Max lines.") {
			t.Error("expected optional parameter")
		}
	})
}

func TestBuildFeedbackPrompt(t *testing.T) {
	a := &Agent{
		strings: EnglishStrings(),
	}

	calls := []toolCall{
		{Tool: "bash", Input: json.RawMessage(`{"command": "ls"}`)},
		{Tool: "read", Input: json.RawMessage(`{"path": "/tmp/test.txt"}`)},
	}

	t.Run("success results use tool_result tags", func(t *testing.T) {
		results := []toolResult{
			{Tool: "bash", Output: json.RawMessage(`{"status": "success"}`)},
			{Tool: "read", Output: json.RawMessage(`{"content": "hello"}`)},
		}

		prompt := a.buildFeedbackPrompt(calls, results, false)

		// Check that tool_result tags are used with name attribute
		if !strings.Contains(prompt, `<tool_result name="bash">`) {
			t.Error("expected <tool_result name=\"bash\"> tag in prompt")
		}
		if !strings.Contains(prompt, `<tool_result name="read">`) {
			t.Error("expected <tool_result name=\"read\"> tag in prompt")
		}
		if !strings.Contains(prompt, `</tool_result>`) {
			t.Error("expected </tool_result> closing tag in prompt")
		}
		if !strings.Contains(prompt, a.strings.FinalResponsePrompt) {
			t.Error("expected final response prompt for successful results")
		}
	})

	t.Run("error results include input and error", func(t *testing.T) {
		results := []toolResult{
			{Tool: "bash", Error: "command not found"},
			{Tool: "read", Output: json.RawMessage(`{"content": "hello"}`)},
		}

		prompt := a.buildFeedbackPrompt(calls[:1], results[:1], true)

		if !strings.Contains(prompt, `<tool_result name="bash">`) {
			t.Error("expected <tool_result name=\"bash\"> tag in error prompt")
		}
		if !strings.Contains(prompt, "### Error: command not found") {
			t.Error("expected error message in prompt")
		}
		if strings.Contains(prompt, a.strings.FinalResponsePrompt) {
			t.Error("should not include final response prompt when there are errors")
		}
	})
}
