package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolParameter describes a single input parameter for a tool.
type ToolParameter struct {
	// Name is the parameter name as it appears in the JSON input.
	Name string
	// Type is the parameter type (e.g., "string", "int", "bool").
	Type string
	// Required indicates whether the parameter must be provided.
	Required bool
	// Description explains what the parameter does.
	Description string
}

// Tool represents an action that an agent can invoke.
type Tool interface {
	// Name returns the unique identifier for this tool.
	Name() string
	// Description returns a human-readable explanation of what this tool does.
	Description() string
	// Parameters returns the structured input parameter definitions.
	Parameters() []ToolParameter
	// Enabled indicates whether this tool is currently available for use.
	Enabled() bool
	// Execute runs the tool with the given input and returns the result.
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// FormatToolParameters formats tool parameters into a human-readable string.
func FormatToolParameters(params []ToolParameter) string {
	if len(params) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("  Parameters:\n")
	for _, p := range params {
		req := "optional"
		if p.Required {
			req = "required"
		}
		sb.WriteString(fmt.Sprintf("    - %s (%s, %s): %s\n", p.Name, p.Type, req, p.Description))
	}
	return sb.String()
}
