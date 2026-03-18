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
	// Enabled indicates whether this tool is currently available for use.
	Enabled() bool
	// Execute runs the tool with the given input and returns the result.
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}
