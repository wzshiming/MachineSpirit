package session

import (
	"context"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// End-to-end test: planner -> skill invocation -> composer -> session manager envelope.
func TestSessionE2ESkillInvocation(t *testing.T) {
	skillOutputs := make(chan string, 1)

	skills := agent.SkillInvoker{
		Skills: map[string]agent.Skill{
			"echo": agent.SkillFunc(func(ctx context.Context, payload map[string]any) (string, error) {
				val, _ := payload["text"].(string)
				skillOutputs <- val
				return "echo:" + val, nil
			}),
		},
	}

	planner := skillPlanner{}

	manager := NewManager(agent.Loop{
		Planner:     planner,
		ToolInvoker: skills,
		Composer:    agent.SimpleComposer{},
	})

	event := model.Event{
		SessionID: "session-e2e",
		Content:   "run-skill",
		Timestamp: time.Now(),
	}

	envelope, err := manager.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if len(envelope.Messages) != 1 {
		t.Fatalf("expected one assistant reply, got %d", len(envelope.Messages))
	}
	if envelope.Messages[0].Content != "run-skill\necho:run-skill" {
		t.Fatalf("unexpected reply content: %q", envelope.Messages[0].Content)
	}

	select {
	case v := <-skillOutputs:
		if v != "run-skill" {
			t.Fatalf("skill received wrong payload: %q", v)
		}
	default:
		t.Fatalf("skill was not invoked")
	}
}

type skillPlanner struct{}

func (skillPlanner) Plan(_ context.Context, input agent.Input) (agent.Plan, error) {
	return agent.Plan{
		Summary: input.Event.Content,
		ToolCalls: []agent.ToolCall{
			{
				Name: "echo",
				Payload: map[string]any{
					"text": input.Event.Content,
				},
			},
		},
	}, nil
}
