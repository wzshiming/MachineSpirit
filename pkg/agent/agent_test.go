package agent

import (
	"encoding/json"
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
		if !containsString(prompt, `<tool_result name="bash">`) {
			t.Error("expected <tool_result name=\"bash\"> tag in prompt")
		}
		if !containsString(prompt, `<tool_result name="read">`) {
			t.Error("expected <tool_result name=\"read\"> tag in prompt")
		}
		if !containsString(prompt, `</tool_result>`) {
			t.Error("expected </tool_result> closing tag in prompt")
		}
		if !containsString(prompt, a.strings.FinalResponsePrompt) {
			t.Error("expected final response prompt for successful results")
		}
	})

	t.Run("error results include input and error", func(t *testing.T) {
		results := []toolResult{
			{Tool: "bash", Error: "command not found"},
			{Tool: "read", Output: json.RawMessage(`{"content": "hello"}`)},
		}

		prompt := a.buildFeedbackPrompt(calls[:1], results[:1], true)

		if !containsString(prompt, `<tool_result name="bash">`) {
			t.Error("expected <tool_result name=\"bash\"> tag in error prompt")
		}
		if !containsString(prompt, "### Error: command not found") {
			t.Error("expected error message in prompt")
		}
		if containsString(prompt, a.strings.FinalResponsePrompt) {
			t.Error("should not include final response prompt when there are errors")
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findString(s, substr)
}

func findString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
