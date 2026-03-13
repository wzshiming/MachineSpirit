package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

func TestHandleEventCreatesSessionAndUpdatesTranscript(t *testing.T) {
	manager := NewManager(nil, WithMaxPending(2), WithPruneAfter(5*time.Minute))

	event := model.Event{
		SessionID: "session-a",
		Content:   "hello there",
		Timestamp: time.Now(),
	}

	envelope, err := manager.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if envelope.Dropped {
		t.Fatalf("expected event to be processed, got dropped: %+v", envelope)
	}
	if len(envelope.Messages) != 1 {
		t.Fatalf("expected a single assistant message, got %d", len(envelope.Messages))
	}
	if len(envelope.Presence) != 2 {
		t.Fatalf("expected typing and active presence updates, got %d", len(envelope.Presence))
	}
	if envelope.Presence[0].Status != model.PresenceTyping || envelope.Presence[1].Status != model.PresenceActive {
		t.Fatalf("unexpected presence sequence: %+v", envelope.Presence)
	}

	snapshot, ok := manager.Snapshot(event.SessionID)
	if !ok {
		t.Fatalf("session was not stored")
	}
	if len(snapshot.Transcript) != 2 {
		t.Fatalf("expected transcript to hold user and assistant messages, got %d entries", len(snapshot.Transcript))
	}
	if snapshot.Transcript[0].Role != model.RoleUser || snapshot.Transcript[1].Role != model.RoleAssistant {
		t.Fatalf("unexpected transcript roles: %+v", snapshot.Transcript)
	}
}

func TestHandleEventDropsWhenOverloaded(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	agentImpl := stubAgent{
		respond: func(ctx context.Context, input agent.Input) (model.Message, error) {
			close(started)
			<-release
			return model.Message{Content: "done"}, nil
		},
	}

	manager := NewManager(agentImpl, WithMaxPending(1))
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = manager.HandleEvent(ctx, model.Event{SessionID: "session-b", Content: "first", Timestamp: time.Now()})
	}()

	<-started

	envelope, err := manager.HandleEvent(ctx, model.Event{SessionID: "session-b", Content: "second", Timestamp: time.Now()})
	if !errors.Is(err, ErrSessionOverloaded) {
		t.Fatalf("expected ErrSessionOverloaded, got %v", err)
	}
	if !envelope.Dropped || envelope.DropReason == "" {
		t.Fatalf("expected drop envelope, got %+v", envelope)
	}

	close(release)
	wg.Wait()
}

func TestPruneInactiveRemovesOldSessions(t *testing.T) {
	now := time.Now()
	clock := func() time.Time {
		return now
	}

	manager := NewManager(stubAgent{
		respond: func(ctx context.Context, input agent.Input) (model.Message, error) {
			return model.Message{Content: "ok"}, nil
		},
	}, WithPruneAfter(time.Minute), WithClock(clock))

	_, err := manager.HandleEvent(context.Background(), model.Event{SessionID: "session-c", Content: "hello", Timestamp: now})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	now = now.Add(2 * time.Minute)
	removed := manager.PruneInactive()
	if len(removed) != 1 || removed[0] != "session-c" {
		t.Fatalf("expected session-c to be pruned, got %v", removed)
	}

	if _, ok := manager.Snapshot("session-c"); ok {
		t.Fatalf("session should have been pruned")
	}
}

type stubAgent struct {
	respond func(ctx context.Context, input agent.Input) (model.Message, error)
}

func (s stubAgent) Respond(ctx context.Context, input agent.Input) (model.Message, error) {
	return s.respond(ctx, input)
}
