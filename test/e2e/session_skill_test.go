package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/model"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// End-to-end: session manager routes an event through planner -> skill -> composer.
func TestSessionSkillInvocationEndToEnd(t *testing.T) {
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

	manager := session.NewManager(agent.Loop{
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

	if envelope.Dropped {
		t.Fatalf("event was dropped: %+v", envelope)
	}

	if len(envelope.Presence) != 2 || envelope.Presence[0].Status != model.PresenceTyping || envelope.Presence[1].Status != model.PresenceActive {
		t.Fatalf("unexpected presence updates: %+v", envelope.Presence)
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

	snapshot, ok := manager.Snapshot(event.SessionID)
	if !ok {
		t.Fatalf("snapshot missing")
	}
	if snapshot.Transcript == nil || len(snapshot.Transcript) != 2 {
		t.Fatalf("expected 2 transcript entries, got %d", len(snapshot.Transcript))
	}
	if snapshot.Transcript[0].Role != model.RoleUser || snapshot.Transcript[1].Role != model.RoleAssistant {
		t.Fatalf("unexpected transcript roles: %+v", snapshot.Transcript)
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
