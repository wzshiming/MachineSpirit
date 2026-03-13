package agent

import (
	"context"
	"testing"
)

func TestMultiToolInvokerDispatchesByKind(t *testing.T) {
	skill := &stubCallInvoker{output: "skill"}
	mcp := &stubCallInvoker{output: "mcp"}
	builtin := &stubCallInvoker{output: "builtin"}
	def := &stubCallInvoker{output: "default"}

	invoker := MultiToolInvoker{
		Skills:   skill,
		MCP:      mcp,
		Builtins: builtin,
		Default:  def,
	}

	plan := Plan{
		ToolCalls: []ToolCall{
			{Kind: ToolKindSkill, Name: "s1"},
			{Kind: ToolKindMCP, Name: "m1"},
			{Kind: ToolKindBuiltin, Name: "b1"},
			{Kind: "unknown", Name: "u1"},
			{Name: "implicit-skill"},
		},
	}

	results, err := invoker.Invoke(context.Background(), plan)
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if len(results) != len(plan.ToolCalls) {
		t.Fatalf("expected %d results, got %d", len(plan.ToolCalls), len(results))
	}

	expectOutputs := []string{"skill", "mcp", "builtin", "default", "skill"}
	for i, res := range results {
		if res.Output != expectOutputs[i] {
			t.Fatalf("result %d output mismatch, expected %q got %q", i, expectOutputs[i], res.Output)
		}
	}
}

type stubCallInvoker struct {
	output string
}

func (s *stubCallInvoker) InvokeCall(ctx context.Context, call ToolCall) ToolResult {
	return ToolResult{
		Name:   call.Name,
		Output: s.output,
	}
}
