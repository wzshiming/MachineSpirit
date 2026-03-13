package agent

import (
	"context"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/memory"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

func TestLoopRecordsMemoryWhenConfigured(t *testing.T) {
	store := &stubMemoryStore{data: make(map[memory.Layer][]string)}

	loop := Loop{
		Planner:     EchoPlanner{},
		ToolInvoker: NoopToolInvoker{},
		Composer:    SimpleComposer{},
		Memory: MemoryAdapter{
			Store: store,
		},
	}

	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	input := Input{
		Event: model.Event{
			SessionID: "s1",
			Content:   "remember me",
			Timestamp: now,
		},
		Transcript: nil,
	}

	msg, err := loop.Respond(context.Background(), input)
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if msg.Content == "" {
		t.Fatalf("expected non-empty reply")
	}

	full := store.data[memory.LayerFullConversations]
	if len(full) != 2 {
		t.Fatalf("expected 2 entries in full conversation, got %d", len(full))
	}
	recent := store.data[memory.LayerRecent]
	if len(recent) != 2 {
		t.Fatalf("expected 2 entries in recent, got %d", len(recent))
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
