package agent

import (
	"context"
	"encoding/json"
)

// Tool represents an action that an agent can invoke.
type Tool interface {
	// Name returns the unique identifier for this tool.
	Name() string
	// Description returns a human-readable explanation of what this tool does.
	Description() string
	// ParametersSchema returns the JSON schema for the tool's input parameters.
	ParametersSchema() map[string]interface{}
	// Execute runs the tool with the given input and returns the result.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// ToolCall represents a request to invoke a specific tool.
type ToolCall struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
}

// ToolResult captures the outcome of a tool execution.
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}
