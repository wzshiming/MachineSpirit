package skills

import (
	"context"
	"errors"
)

// Invoker resolves a skill then executes it.
type Invoker struct {
	Selector Selector
}

// Invoke runs a named skill using the selector.
func (i Invoker) Invoke(ctx context.Context, name string, payload map[string]any) (string, error) {
	skill, ok := i.Selector.Select(name)
	if !ok || skill == nil {
		return "", errors.New("skill not found")
	}
	return skill.Invoke(ctx, payload)
}
