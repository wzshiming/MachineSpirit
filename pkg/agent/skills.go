package agent

import (
	"context"
	"errors"
)

// Skill represents an executable capability the agent can call.
type Skill interface {
	Invoke(ctx context.Context, payload map[string]any) (string, error)
}

// SkillFunc is a function adapter to satisfy the Skill interface.
type SkillFunc func(ctx context.Context, payload map[string]any) (string, error)

func (f SkillFunc) Invoke(ctx context.Context, payload map[string]any) (string, error) {
	return f(ctx, payload)
}

// SkillInvoker dispatches tool calls to registered skills.
type SkillInvoker struct {
	Skills map[string]Skill
}

func (s SkillInvoker) Invoke(ctx context.Context, plan Plan) ([]ToolResult, error) {
	if len(plan.ToolCalls) == 0 {
		return nil, nil
	}

	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		skill, ok := s.Skills[call.Name]
		if !ok || skill == nil {
			results = append(results, ToolResult{
				Name: call.Name,
				Err:  errors.New("skill not found"),
			})
			continue
		}
		output, err := skill.Invoke(ctx, call.Payload)
		results = append(results, ToolResult{
			Name:   call.Name,
			Output: output,
			Err:    err,
		})
	}
	return results, nil
}
