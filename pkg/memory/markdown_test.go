package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownStoreReadWriteLayers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mem.md")
	store := MarkdownStore{Path: path}

	// Missing file yields empty layer.
	if vals, err := store.Read(context.Background(), LayerCoreLongTerm); err != nil || len(vals) != 0 {
		t.Fatalf("expected empty read on missing file, got %v %v", vals, err)
	}

	core := []string{"persistent fact", "rule of thumb"}
	if err := store.Write(context.Background(), LayerCoreLongTerm, core); err != nil {
		t.Fatalf("write core failed: %v", err)
	}

	recent := []string{"recent insight"}
	if err := store.Write(context.Background(), LayerRecent, recent); err != nil {
		t.Fatalf("write recent failed: %v", err)
	}

	gotCore, _ := store.Read(context.Background(), LayerCoreLongTerm)
	if len(gotCore) != len(core) || gotCore[0] != core[0] {
		t.Fatalf("core round-trip mismatch: %+v", gotCore)
	}

	gotRecent, _ := store.Read(context.Background(), LayerRecent)
	if len(gotRecent) != len(recent) || gotRecent[0] != recent[0] {
		t.Fatalf("recent round-trip mismatch: %+v", gotRecent)
	}

	// Ensure other layers remain intact after updating another layer.
	summaries := []string{"2026-03-12: summarized"}
	if err := store.Write(context.Background(), LayerDailySummaries, summaries); err != nil {
		t.Fatalf("write summaries failed: %v", err)
	}
	gotRecentAfter, _ := store.Read(context.Background(), LayerRecent)
	if gotRecentAfter[0] != recent[0] {
		t.Fatalf("recent mutated unexpectedly: %+v", gotRecentAfter)
	}
}

func TestMarkdownStoreOnDemandReads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mem.md")
	store := MarkdownStore{Path: path}

	// Write once.
	if err := store.Write(context.Background(), LayerFullConversations, []string{"turn1"}); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Manually edit file to simulate external update.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	edited := string(content)
	edited = edited + "- turn2\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("manual write: %v", err)
	}

	// On-demand read should pick up the manual edit.
	got, err := store.Read(context.Background(), LayerFullConversations)
	if err != nil {
		t.Fatalf("read after edit failed: %v", err)
	}
	if len(got) != 2 || got[1] != "turn2" {
		t.Fatalf("expected manual edit to be visible, got %+v", got)
	}
}
