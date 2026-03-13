package agent

import (
	"context"
	"strings"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/memory"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// Memory captures how agents persist turns.
type Memory interface {
	RecordTurn(ctx context.Context, sessionID string, timestamp time.Time, userContent string, assistant model.Message)
}

// MemoryAdapter writes agent turns into a memory.Store.
type MemoryAdapter struct {
	Store memory.Store
}

func (m MemoryAdapter) RecordTurn(ctx context.Context, sessionID string, timestamp time.Time, userContent string, assistant model.Message) {
	if m.Store == nil {
		return
	}

	userMsg := model.Message{
		Role:      model.RoleUser,
		Content:   userContent,
		Timestamp: timestamp,
	}

	userEntry := formatMemoryLine(sessionID, userMsg)
	replyEntry := formatMemoryLine(sessionID, assistant)

	appendLayer(ctx, m.Store, memory.LayerFullConversations, userEntry, replyEntry)
	appendCappedLayer(ctx, m.Store, memory.LayerRecent, recentMemoryLimit, userEntry, replyEntry)
}

func appendLayer(ctx context.Context, store memory.Store, layer memory.Layer, entries ...string) {
	cur, err := store.Read(ctx, layer)
	if err != nil {
		return
	}
	cur = append(cur, entries...)
	_ = store.Write(ctx, layer, cur)
}

const recentMemoryLimit = 50

func appendCappedLayer(ctx context.Context, store memory.Store, layer memory.Layer, capSize int, entries ...string) {
	cur, err := store.Read(ctx, layer)
	if err != nil {
		return
	}
	cur = append(cur, entries...)
	if len(cur) > capSize {
		cur = cur[len(cur)-capSize:]
	}
	_ = store.Write(ctx, layer, cur)
}

func formatMemoryLine(sessionID string, msg model.Message) string {
	ts := msg.Timestamp.UTC().Format(time.RFC3339)
	return sessionID + "|" + ts + "|" + string(msg.Role) + "|" + strings.ReplaceAll(msg.Content, "\n", " ")
}
