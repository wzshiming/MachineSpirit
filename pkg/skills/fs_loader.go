package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// LoadDir builds a registry by scanning a directory of markdown skill files.
// Each file becomes a skill named by its basename (sans extension) with the
// description sourced from the first non-empty line.
func LoadDir(dir string) (Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Registry{}, err
	}

	skillsList := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		skill, err := newMarkdownSkill(path)
		if err != nil {
			continue
		}
		skillsList = append(skillsList, skill)
	}

	return NewRegistry(skillsList...), nil
}

type markdownSkill struct {
	name        string
	description string
	path        string
}

func newMarkdownSkill(path string) (markdownSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return markdownSkill{}, err
	}
	lines := strings.Split(string(data), "\n")
	var firstLine string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		// Remove leading Markdown heading markers for description.
		firstLine = strings.TrimLeft(trim, "# ")
		break
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return markdownSkill{
		name:        name,
		description: firstLine,
		path:        path,
	}, nil
}

func (m markdownSkill) Name() string {
	return m.name
}

func (m markdownSkill) Description() string {
	return m.description
}

// Invoke returns the entire markdown content.
func (m markdownSkill) Invoke(ctx context.Context, payload map[string]any) (string, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
