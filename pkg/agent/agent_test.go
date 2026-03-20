package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// stubLLM is a minimal LLM for testing that echoes back the prompt.
type stubLLM struct{}

func (s *stubLLM) Complete(ctx context.Context, req llm.ChatRequest) (llm.Message, error) {
	return llm.Message{
		Role:      llm.RoleAssistant,
		Content:   "done: " + req.Prompt.Content,
		Timestamp: time.Unix(1, 0),
	}, nil
}

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedCalls      []toolCall
		expectedNonTool    string
	}{
		{
			name:            "no tool calls",
			input:           "Just a regular response with no tool calls.",
			expectedCalls:   nil,
			expectedNonTool: "Just a regular response with no tool calls.",
		},
		{
			name:  "single tool call",
			input: `Some text <tool_call name="bash">{"command": "ls"}</tool_call> more text`,
			expectedCalls: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
			},
			expectedNonTool: "Some text  more text",
		},
		{
			name: "multiple tool calls",
			input: `<tool_call name="bash">{"command": "ls"}</tool_call>
<tool_call name="read">{"path": "/tmp/test.txt"}</tool_call>`,
			expectedCalls: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
				{Tool: "read", Input: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
			expectedNonTool: "",
		},
		{
			name:            "missing name attribute",
			input:           `<tool_call>{"command": "ls"}</tool_call>`,
			expectedCalls:   nil,
			expectedNonTool: "",
		},
		{
			name:            "missing closing tag",
			input:           `<tool_call name="bash">{"command": "ls"}`,
			expectedCalls:   nil,
			expectedNonTool: `<tool_call name="bash">{"command": "ls"}`,
		},
		{
			name: "tool_call in code string literal",
			input: `start := strings.Index(response, "<tool_call\")\n` +
				`\t\tif start == -1 {\n` +
				`\t\t\tnonToolContent.WriteString(response)\n` +
				`\t\t\tbreak\n` +
				`\t\t}\n\n` +
				`\t\tnonToolContent.WriteString(response[:start])\n\n` +
				`\t\ttagEnd := strings.Index(response[start:], \">")`,
			expectedCalls:   nil,
			expectedNonTool: `start := strings.Index(response, "<tool_call\")\n\t\tif start == -1 {\n\t\t\tnonToolContent.WriteString(response)\n\t\t\tbreak\n\t\t}\n\n\t\tnonToolContent.WriteString(response[:start])\n\n\t\ttagEnd := strings.Index(response[start:], \">")`,
		},
		{
			name:            "tool_call followed by non-space non-gt",
			input:           `<tool_call_extra name="bash">{"command": "ls"}</tool_call_extra>`,
			expectedCalls:   nil,
			expectedNonTool: `<tool_call_extra name="bash">{"command": "ls"}</tool_call_extra>`,
		},
		{
			name: "valid tool call after false match in code",
			input: `Here is code: "<tool_call\")` + "\n" +
				`And here is a real call: <tool_call name="bash">{"command": "ls"}</tool_call>`,
			expectedCalls: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
			},
			expectedNonTool: `Here is code: "<tool_call\")` + "\n" + `And here is a real call:`,
		},
		{
			name:  "whitespace around JSON",
			input: `<tool_call name="bash">  {"command": "ls"}  </tool_call>`,
			expectedCalls: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
			},
			expectedNonTool: "",
		},
		{
			name:  "non-tool text before and after tool call",
			input: "I'll help you.\n<tool_call name=\"bash\">{\"command\": \"ls\"}</tool_call>\nLet me know if you need more.",
			expectedCalls: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
			},
			expectedNonTool: "I'll help you.\n\nLet me know if you need more.",
		},
		{
			name:  "text between multiple tool calls",
			input: "First:\n<tool_call name=\"bash\">{\"command\": \"ls\"}</tool_call>\nThen:\n<tool_call name=\"read\">{\"path\": \"/tmp/a\"}</tool_call>\nDone.",
			expectedCalls: []toolCall{
				{Tool: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
				{Tool: "read", Input: json.RawMessage(`{"path":"/tmp/a"}`)},
			},
			expectedNonTool: "First:\n\nThen:\n\nDone.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCalls(tt.input)
			if len(result.ToolCalls) != len(tt.expectedCalls) {
				t.Fatalf("expected %d tool calls, got %d", len(tt.expectedCalls), len(result.ToolCalls))
			}
			for i, call := range result.ToolCalls {
				if call.Tool != tt.expectedCalls[i].Tool {
					t.Errorf("call %d: expected tool %q, got %q", i, tt.expectedCalls[i].Tool, call.Tool)
				}
				if string(call.Input) != string(tt.expectedCalls[i].Input) {
					t.Errorf("call %d: expected input %s, got %s", i, tt.expectedCalls[i].Input, call.Input)
				}
			}
			if result.NonToolContent != tt.expectedNonTool {
				t.Errorf("non-tool content: expected %q, got %q", tt.expectedNonTool, result.NonToolContent)
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

func TestMaybeAutoCompress(t *testing.T) {
	tmpDir := t.TempDir()
	pm, err := persistence.NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	provider := &stubLLM{}
	sess := session.NewSession(provider,
		session.WithPersistenceManager(pm),
		session.WithSave("auto-compress-test"),
	)

	ctx := context.Background()
	// Add enough messages to exceed a low threshold
	for i := range 10 {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}
	// 10 exchanges = 20 messages
	if sess.Size() != 20 {
		t.Fatalf("expected 20 messages, got %d", sess.Size())
	}

	ag, err := NewAgent(sess,
		WithPersistenceManager(pm),
		WithCompressThreshold(15), // Set threshold lower than current size
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// maybeAutoCompress should compress because size (20) > threshold (15)
	ag.maybeAutoCompress(ctx)

	if sess.Size() >= 20 {
		t.Errorf("expected transcript to be compressed below 20 messages, got %d", sess.Size())
	}
}

func TestMaybeAutoCompressNoopBelowThreshold(t *testing.T) {
	provider := &stubLLM{}
	sess := session.NewSession(provider)

	ctx := context.Background()
	// Add a few messages
	for i := range 3 {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}
	originalSize := sess.Size() // 6 messages

	ag, err := NewAgent(sess,
		WithCompressThreshold(50), // Threshold much higher than current size
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ag.maybeAutoCompress(ctx)

	if sess.Size() != originalSize {
		t.Errorf("expected no compression, size should remain %d, got %d", originalSize, sess.Size())
	}
}

func TestMaybeAutoCompressDisabled(t *testing.T) {
	provider := &stubLLM{}
	sess := session.NewSession(provider)

	ctx := context.Background()
	for i := range 5 {
		_, err := sess.Complete(ctx, llm.ChatRequest{
			Prompt: llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("message %d", i)},
		})
		if err != nil {
			t.Fatalf("Complete returned error: %v", err)
		}
	}
	originalSize := sess.Size()

	ag, err := NewAgent(sess,
		WithCompressThreshold(0), // Disabled
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ag.maybeAutoCompress(ctx)

	if sess.Size() != originalSize {
		t.Errorf("expected no compression when disabled, size should remain %d, got %d", originalSize, sess.Size())
	}
}

func TestWithCompressThresholdDefault(t *testing.T) {
	provider := &stubLLM{}
	sess := session.NewSession(provider)

	ag, err := NewAgent(sess)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	if ag.compressThreshold != defaultCompressThreshold {
		t.Errorf("expected default compress threshold %d, got %d", defaultCompressThreshold, ag.compressThreshold)
	}
}

func TestWithCompressThresholdCustom(t *testing.T) {
	provider := &stubLLM{}
	sess := session.NewSession(provider)

	ag, err := NewAgent(sess, WithCompressThreshold(100))
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	if ag.compressThreshold != 100 {
		t.Errorf("expected compress threshold 100, got %d", ag.compressThreshold)
	}
}
