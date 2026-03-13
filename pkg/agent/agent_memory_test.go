package agent

import (
	"context"
	"reflect"
	"strconv"
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

func TestLoopLoadsMemoriesBeforePlanning(t *testing.T) {
	expected := MemoryContext{
		CoreLongTerm:      []string{"core 1"},
		Recent:            []string{"s1|t|user|hi"},
		DailySummaries:    []string{"summary"},
		FullConversations: []string{"s1|t|assistant|ok"},
	}

	spyPlanner := &capturePlanner{}
	spyMemory := &spyMemory{context: expected}

	loop := Loop{
		Planner:     spyPlanner,
		ToolInvoker: NoopToolInvoker{},
		Composer:    SimpleComposer{},
		Memory:      spyMemory,
	}

	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	_, err := loop.Respond(context.Background(), Input{
		Event: model.Event{
			SessionID: "s1",
			Content:   "hello",
			Timestamp: now,
		},
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	if !reflect.DeepEqual(spyPlanner.lastInput.Memories, expected) {
		t.Fatalf("expected memories %+v, got %+v", expected, spyPlanner.lastInput.Memories)
	}
	if !spyMemory.recorded {
		t.Fatalf("expected memory recorder invoked")
	}
}

func TestLoopChainsNextPlan(t *testing.T) {
	var invoked int
	planner := chainPlanner{Steps: 2}
	invoker := &countingInvoker{invokes: &invoked}

	loop := Loop{
		Planner:     planner,
		ToolInvoker: invoker,
		Composer:    SimpleComposer{},
	}

	msg, err := loop.Respond(context.Background(), Input{
		Event: model.Event{
			SessionID: "s-next",
			Content:   "go",
			Timestamp: time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}
	if msg.Content != "step-2" {
		t.Fatalf("expected final content step-2, got %q", msg.Content)
	}
	if invoked != 2 {
		t.Fatalf("expected 2 invocations, got %d", invoked)
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

type capturePlanner struct {
	lastInput Input
}

func (c *capturePlanner) Plan(ctx context.Context, input Input) (Plan, error) {
	c.lastInput = input
	return Plan{Summary: "ok"}, nil
}

type spyMemory struct {
	context  MemoryContext
	recorded bool
}

func (s *spyMemory) RecordTurn(ctx context.Context, sessionID string, user model.Message, assistant model.Message) {
	s.recorded = true
}

func (s *spyMemory) Load(ctx context.Context, sessionID string) MemoryContext {
	return s.context
}

type chainPlanner struct {
	Steps int
}

func (p chainPlanner) Plan(ctx context.Context, input Input) (Plan, error) {
	var head *Plan
	for i := p.Steps; i >= 1; i-- {
		head = &Plan{
			Summary: "step-" + strconv.Itoa(i),
			Next:    head,
		}
	}
	if head == nil {
		head = &Plan{Summary: "step-0"}
	}
	return *head, nil
}

type countingInvoker struct {
	invokes *int
}

func (c *countingInvoker) Invoke(ctx context.Context, plan Plan) ([]ToolResult, error) {
	if c.invokes != nil {
		*c.invokes++
	}
	return nil, nil
}
