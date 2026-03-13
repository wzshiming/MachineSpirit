package agent

import (
	"context"
	"strings"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// Agent consumes session context and produces a reply message.
type Agent interface {
	Respond(ctx context.Context, input Input) (model.Message, error)
}

// Input carries the inbound event and current transcript state.
type Input struct {
	Event      model.Event
	Transcript []model.Message
	Memories   MemoryContext
}

// Plan holds the high-level intent and tool calls for a turn.
type Plan struct {
	Summary   string
	ToolCalls []ToolCall
}

// ToolKind indicates which subsystem should execute the tool.
type ToolKind string

const (
	ToolKindSkill   ToolKind = "skill"
	ToolKindMCP     ToolKind = "mcp"
	ToolKindBuiltin ToolKind = "builtin"
)

// ToolCall represents a requested tool invocation.
type ToolCall struct {
	Kind    ToolKind
	Name    string
	Payload map[string]any
}

// ToolResult is the outcome of a tool invocation.
type ToolResult struct {
	Name   string
	Output string
	Err    error
}

// Planner produces a plan for the current turn.
type Planner interface {
	Plan(ctx context.Context, input Input) (Plan, error)
}

// ToolInvoker executes tool calls described in the plan.
type ToolInvoker interface {
	Invoke(ctx context.Context, plan Plan) ([]ToolResult, error)
}

// Composer builds the assistant reply using the plan and tool results.
type Composer interface {
	Compose(ctx context.Context, plan Plan, results []ToolResult) (model.Message, error)
}

// Loop wires planner, tool invoker, and composer into a single Agent.
type Loop struct {
	Planner     Planner
	ToolInvoker ToolInvoker
	Composer    Composer
	Memory      Memory
}

func (a Loop) Respond(ctx context.Context, input Input) (model.Message, error) {
	if a.Memory != nil {
		input.Memories = a.Memory.Load(ctx, input.Event.SessionID)
	}

	plan, err := a.Planner.Plan(ctx, input)
	if err != nil {
		return model.Message{}, err
	}

	results, err := a.ToolInvoker.Invoke(ctx, plan)
	if err != nil {
		return model.Message{}, err
	}

	msg, err := a.Composer.Compose(ctx, plan, results)
	if err != nil {
		return model.Message{}, err
	}

	if msg.Role == "" {
		msg.Role = model.RoleAssistant
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	if a.Memory != nil {
		a.Memory.RecordTurn(ctx, input.Event.SessionID, input.Event.Timestamp, input.Event.Content, msg)
	}

	return msg, nil
}

// EchoPlanner turns the inbound message into a minimal plan.
type EchoPlanner struct{}

func (EchoPlanner) Plan(_ context.Context, input Input) (Plan, error) {
	content := strings.TrimSpace(input.Event.Content)
	summary := content
	if summary == "" {
		summary = "acknowledge event"
	}
	return Plan{
		Summary: summary,
	}, nil
}

// NoopToolInvoker executes no tools but keeps the streaming contract intact.
type NoopToolInvoker struct{}

func (NoopToolInvoker) Invoke(_ context.Context, plan Plan) ([]ToolResult, error) {
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		results = append(results, ToolResult{Name: call.Name})
	}
	return results, nil
}

// SimpleComposer echoes the plan summary and any tool outputs.
type SimpleComposer struct{}

func (SimpleComposer) Compose(_ context.Context, plan Plan, results []ToolResult) (model.Message, error) {
	var fragments []string
	if plan.Summary != "" {
		fragments = append(fragments, plan.Summary)
	}
	for _, res := range results {
		if res.Output != "" {
			fragments = append(fragments, res.Output)
		}
	}
	content := strings.Join(fragments, "\n")
	if content == "" {
		content = "ok"
	}
	return model.Message{
		Role:      model.RoleAssistant,
		Content:   content,
		Timestamp: time.Now(),
	}, nil
}
