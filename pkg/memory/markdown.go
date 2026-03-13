package memory

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Layer identifies a logical memory layer.
type Layer string

const (
	LayerCoreLongTerm      Layer = "Core Memory (Long Term)"
	LayerRecent            Layer = "Recent Memory"
	LayerDailySummaries    Layer = "Historical Daily Memory Summaries"
	LayerFullConversations Layer = "Historical Full Conversation Content"
)

var layerHeadings = []Layer{
	LayerCoreLongTerm,
	LayerRecent,
	LayerDailySummaries,
	LayerFullConversations,
}

// MarkdownStore persists memories into a markdown file with explicit layer headings.
// Reads are performed on-demand; no state is cached between calls.
type MarkdownStore struct {
	Path string
}

// Read returns the entries for the requested layer. If the file or layer is missing,
// an empty slice is returned without error.
func (s MarkdownStore) Read(_ context.Context, layer Layer) ([]string, error) {
	sections, err := s.readAll()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return append([]string(nil), sections[layer]...), nil
}

// Write replaces the entries for the requested layer and persists the file.
func (s MarkdownStore) Write(_ context.Context, layer Layer, entries []string) error {
	sections, err := s.readAll()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sections[layer] = append([]string(nil), entries...)
	return s.writeAll(sections)
}

func (s MarkdownStore) readAll() (map[Layer][]string, error) {
	sections := make(map[Layer][]string, len(layerHeadings))
	f, err := os.Open(s.Path)
	if err != nil {
		return sections, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var current Layer
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			for _, l := range layerHeadings {
				if title == string(l) {
					current = l
					break
				}
			}
			continue
		}
		if current == "" || line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		}
		sections[current] = append(sections[current], line)
	}
	if err := scanner.Err(); err != nil {
		return sections, err
	}
	return sections, nil
}

func (s MarkdownStore) writeAll(sections map[Layer][]string) error {
	if s.Path == "" {
		return errors.New("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for i, layer := range layerHeadings {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## ")
		b.WriteString(string(layer))
		b.WriteString("\n")
		for _, entry := range sections[layer] {
			if strings.TrimSpace(entry) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(entry)
			b.WriteString("\n")
		}
	}
	return os.WriteFile(s.Path, []byte(b.String()), 0o644)
}
