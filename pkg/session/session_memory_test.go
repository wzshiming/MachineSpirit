package session

import (
	"context"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/memory"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

func TestSessionPersistsMemoryLayers(t *testing.T) {
	store := &stubMemoryStore{
		data: make(map[memory.Layer][]string),
	}

	manager := NewManager(agent.Loop{
		Planner:     agent.EchoPlanner{},
		ToolInvoker: agent.NoopToolInvoker{},
		Composer:    agent.SimpleComposer{},
	}, WithMemory(store))

	event := model.Event{
		SessionID: "s1",
		Content:   "hello memory",
		Timestamp: time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC),
	}

	_, err := manager.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	full := store.data[memory.LayerFullConversations]
	if len(full) != 2 {
		t.Fatalf("expected 2 entries in full conversations, got %d", len(full))
	}
	if full[0] == full[1] {
		t.Fatalf("expected distinct user/assistant entries")
	}

	recent := store.data[memory.LayerRecent]
	if len(recent) != 2 {
		t.Fatalf("expected 2 entries in recent, got %d", len(recent))
	}

	summaries := store.data[memory.LayerDailySummaries]
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary entry, got %d", len(summaries))
	}
}

type stubMemoryStore struct {
	data map[memory.Layer][]string
}

func (s *stubMemoryStore) Read(ctx context.Context, layer memory.Layer) ([]string, error) {
	return append([]string(nil), s.data[layer]...), nil
}

func (s *stubMemoryStore) Write(ctx context.Context, layer memory.Layer, entries []string) error {
	s.data[layer] = append([]string(nil), entries...)
	return nil
}
