package agent

import (
	"context"
	"encoding/json"
)

// Skill represents a high-level, composable capability that an agent can use.
// Skills differ from Tools in that they provide richer metadata, can be loaded
// from external sources (like markdown files), and can compose multiple tools.
type Skill interface {
	// Name returns the unique identifier for this skill.
	Name() string
	// Description returns a brief, human-readable explanation of what this skill does.
	Description() string
	// DetailedDescription returns a comprehensive explanation with usage examples.
	DetailedDescription() string
	// ParametersSchema returns the JSON schema for the skill's input parameters.
	ParametersSchema() map[string]interface{}
	// Execute runs the skill with the given input and returns the result.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// SkillCall represents a request to invoke a specific skill.
type SkillCall struct {
	SkillName string          `json:"skill_name"`
	Input     json.RawMessage `json:"input"`
}

// SkillResult captures the outcome of a skill execution.
type SkillResult struct {
	SkillName string `json:"skill_name"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
}

// ToolKind represents the type of tool/skill being invoked.
type ToolKind string

const (
	// ToolKindSkill represents a high-level skill invocation.
	ToolKindSkill ToolKind = "skill"
	// ToolKindTool represents a low-level tool invocation.
	ToolKindTool ToolKind = "tool"
	// ToolKindMCP represents a Model Context Protocol tool.
	ToolKindMCP ToolKind = "mcp"
	// ToolKindBuiltIn represents a built-in system tool.
	ToolKindBuiltIn ToolKind = "built-in"
)

// UnifiedToolCall represents a call to any kind of tool or skill.
type UnifiedToolCall struct {
	Kind     ToolKind        `json:"kind"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	ToolName string          `json:"tool_name,omitempty"` // For backward compatibility
}

// UnifiedToolResult captures the outcome of any tool or skill execution.
type UnifiedToolResult struct {
	Kind   ToolKind `json:"kind"`
	Name   string   `json:"name"`
	Output string   `json:"output"`
	Error  string   `json:"error,omitempty"`
}
