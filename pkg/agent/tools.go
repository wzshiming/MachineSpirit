package agent

import (
	"context"
	"errors"
)

// BuiltinInvoker executes registered built-in tools.
type BuiltinInvoker struct {
	Tools map[string]Skill
}

func (b BuiltinInvoker) InvokeCall(ctx context.Context, call ToolCall) ToolResult {
	tool, ok := b.Tools[call.Name]
	if !ok || tool == nil {
		return ToolResult{
			Name: call.Name,
			Err:  errors.New("builtin tool not found"),
		}
	}
	output, err := tool.Invoke(ctx, call.Payload)
	return ToolResult{
		Name:   call.Name,
		Output: output,
		Err:    err,
	}
}

// MCPInvoker executes tools exposed via MCP.
type MCPInvoker struct {
	Tools map[string]Skill
}

func (m MCPInvoker) InvokeCall(ctx context.Context, call ToolCall) ToolResult {
	tool, ok := m.Tools[call.Name]
	if !ok || tool == nil {
		return ToolResult{
			Name: call.Name,
			Err:  errors.New("mcp tool not found"),
		}
	}
	output, err := tool.Invoke(ctx, call.Payload)
	return ToolResult{
		Name:   call.Name,
		Output: output,
		Err:    err,
	}
}

// MultiToolInvoker routes tool calls to the appropriate invoker based on kind.
type MultiToolInvoker struct {
	Skills   CallInvoker
	MCP      CallInvoker
	Builtins CallInvoker
	Default  CallInvoker
}

func (m MultiToolInvoker) Invoke(ctx context.Context, plan Plan) ([]ToolResult, error) {
	if len(plan.ToolCalls) == 0 {
		return nil, nil
	}

	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		inv := m.invokerFor(call.Kind)
		if inv == nil {
			results = append(results, ToolResult{
				Name: call.Name,
				Err:  errors.New("no invoker for tool kind"),
			})
			continue
		}
		results = append(results, inv.InvokeCall(ctx, call))
	}
	return results, nil
}

func (m MultiToolInvoker) invokerFor(kind ToolKind) CallInvoker {
	switch kind {
	case "", ToolKindSkill:
		if m.Skills != nil {
			return m.Skills
		}
	case ToolKindMCP:
		if m.MCP != nil {
			return m.MCP
		}
	case ToolKindBuiltin:
		if m.Builtins != nil {
			return m.Builtins
		}
	}

	if m.Default != nil {
		return m.Default
	}
	if m.Skills != nil {
		return m.Skills
	}
	return nil
}
