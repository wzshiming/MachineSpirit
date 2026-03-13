package agent

import (
	"context"
	"errors"

	"github.com/wzshiming/MachineSpirit/pkg/skills"
)

// CallInvoker executes a single ToolCall and returns a ToolResult.
type CallInvoker interface {
	InvokeCall(ctx context.Context, call ToolCall) ToolResult
}

// SkillInvoker dispatches tool calls to registered skills using the selector.
type SkillInvoker struct {
	Selector skills.Selector
}

func (s SkillInvoker) Invoke(ctx context.Context, plan Plan) ([]ToolResult, error) {
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		results = append(results, s.InvokeCall(ctx, call))
	}
	return results, nil
}

func (s SkillInvoker) InvokeCall(ctx context.Context, call ToolCall) ToolResult {
	skill, ok := s.Selector.Select(call.Name)
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
