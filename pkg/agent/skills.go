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

// CallInvoker executes a single ToolCall and returns a ToolResult.
type CallInvoker interface {
	InvokeCall(ctx context.Context, call ToolCall) ToolResult
}

// SkillInvoker dispatches tool calls to registered skills.
type SkillInvoker struct {
	Skills map[string]Skill
}

func (s SkillInvoker) Invoke(ctx context.Context, plan Plan) ([]ToolResult, error) {
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		results = append(results, s.InvokeCall(ctx, call))
	}
	return results, nil
}

func (s SkillInvoker) InvokeCall(ctx context.Context, call ToolCall) ToolResult {
	skill, ok := s.Skills[call.Name]
	if !ok || skill == nil {
		return ToolResult{
			Name: call.Name,
			Err:  errors.New("skill not found"),
		}
	}
	output, err := skill.Invoke(ctx, call.Payload)
	return ToolResult{
		Name:   call.Name,
		Output: output,
		Err:    err,
	}
}
