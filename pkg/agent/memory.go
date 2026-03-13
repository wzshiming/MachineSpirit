package agent

import (
	"context"
	"strings"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/memory"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// Memory captures how agents persist turns and load associative memories.
type Memory interface {
	RecordTurn(ctx context.Context, sessionID string, user model.Message, assistant model.Message)
	Load(ctx context.Context, sessionID string) MemoryContext
}

// MemoryAdapter writes agent turns into a memory.Store.
type MemoryAdapter struct {
	Store memory.Store
}

// MemoryContext exposes associative memory slices for planners and composers.
type MemoryContext struct {
	CoreLongTerm      []string
	Recent            []string
	DailySummaries    []string
	FullConversations []string
}

func (m MemoryAdapter) RecordTurn(ctx context.Context, sessionID string, user model.Message, assistant model.Message) {
	if m.Store == nil {
		return
	}

	userEntry := formatMemoryLine(sessionID, user)
	replyEntry := formatMemoryLine(sessionID, assistant)

	appendLayer(ctx, m.Store, memory.LayerFullConversations, userEntry, replyEntry)
	appendCappedLayer(ctx, m.Store, memory.LayerRecent, recentMemoryLimit, userEntry, replyEntry)
}

func (m MemoryAdapter) Load(ctx context.Context, sessionID string) MemoryContext {
	if m.Store == nil {
		return MemoryContext{}
	}

	return MemoryContext{
		CoreLongTerm:      readLayer(ctx, m.Store, memory.LayerCoreLongTerm, sessionID, false),
		Recent:            readLayer(ctx, m.Store, memory.LayerRecent, sessionID, true),
		DailySummaries:    readLayer(ctx, m.Store, memory.LayerDailySummaries, sessionID, false),
		FullConversations: readLayer(ctx, m.Store, memory.LayerFullConversations, sessionID, true),
	}
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

func readLayer(ctx context.Context, store memory.Store, layer memory.Layer, sessionID string, filterBySession bool) []string {
	entries, err := store.Read(ctx, layer)
	if err != nil {
		return nil
	}
	entries = append([]string(nil), entries...)

	if filterBySession && sessionID != "" {
		prefix := sessionID + "|"
		filtered := entries[:0]
		for _, entry := range entries {
			if strings.HasPrefix(entry, prefix) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	return entries
}

func formatMemoryLine(sessionID string, msg model.Message) string {
	ts := msg.Timestamp.UTC().Format(time.RFC3339)
	return sessionID + "|" + ts + "|" + string(msg.Role) + "|" + strings.ReplaceAll(msg.Content, "\n", " ")
}
