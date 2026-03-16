package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// FileMemory stores persistent information and context
	FileMemory = "MEMORY.md"
	// FileIdentity stores agent identity and personality
	FileIdentity = "IDENTITY.md"
	// FileAgents stores information about other agents
	FileAgents = "AGENTS.md"
	// FileSoul stores agent identity and personality
	FileSoul = "SOUL.md"
	// FileUser stores user information and preferences
	FileUser = "USER.md"
	// FileBootstrap stores initial greeting and setup instructions (deleted after boot)
	FileBootstrap = "BOOTSTRAP.md"
	// FileTools stores information about available tools (optional, can be generated from agent configuration)
	FileTools = "TOOLS.md"
)

// PersistenceManager handles loading and saving persistence files
type PersistenceManager struct {
	baseDir string
}

// NewPersistenceManager creates a new persistence manager
// If baseDir is empty, uses the current working directory
func NewPersistenceManager(baseDir string) (*PersistenceManager, error) {
	if baseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		baseDir = cwd
	}

	return &PersistenceManager{
		baseDir: baseDir,
	}, nil
}

// getFilePath returns the full path to a persistence file
func (pm *PersistenceManager) getFilePath(filename string) string {
	return filepath.Join(pm.baseDir, filename)
}

// ReadFile reads the contents of a persistence file
func (pm *PersistenceManager) ReadFile(filename string) (string, error) {
	path := pm.getFilePath(filename)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read %s: %w", filename, err)
	}
	return string(content), nil
}

// DeleteFile deletes a persistence file
func (pm *PersistenceManager) DeleteFile(filename string) error {
	path := pm.getFilePath(filename)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete %s: %w", filename, err)
	}
	return nil
}

// BuildSystemPrompt constructs a system prompt from persistence files
func (pm *PersistenceManager) BuildSystemPrompt(basePrompt string) string {
	var parts []string

	if basePrompt != "" {
		parts = append(parts, basePrompt)
	}

	now := time.Now()

	zone, offset := now.Zone()

	parts = append(parts, fmt.Sprintf("Current time %s, zone %s (UTC%+d)", now.Format(time.RFC3339), zone, offset/3600))

	list := []string{
		FileBootstrap,
		FileSoul,
		FileAgents,
		FileIdentity,
		FileUser,
		FileTools,
		FileMemory,
	}

	for _, item := range list {
		content, err := pm.ReadFile(item)
		if err != nil {
			continue
		}
		if content == "" {
			continue
		}

		path := pm.getFilePath(item)
		meta, content, err := parseMarkdown(content)
		if err == nil {
			parts = append(parts, fmt.Sprintf("# %s (%s): %s\n%s", item, path, meta.Summary, content))
		}
	}

	return strings.Join(parts, "\n\n")
}

type metadata struct {
	Summary  string   `yaml:"summary"`
	ReadWhen []string `yaml:"read_when"`
}

func parseMarkdown(content string) (*metadata, string, error) {
	// Split frontmatter and content
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, "", fmt.Errorf("invalid meta format: missing YAML frontmatter")
	}

	// Parse YAML frontmatter
	var data *metadata
	if err := yaml.Unmarshal([]byte(parts[1]), &data); err != nil {
		return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	return data, parts[2], nil
}
