package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MarkdownSkill represents an instruction-based skill loaded from a markdown file.
// This follows the Anthropic Skills pattern where skills are instructional guides
// rather than executable code.
type MarkdownSkill struct {
	name        string
	description string
	path        string
}

// NewMarkdownSkill creates a skill from markdown content.
func NewMarkdownSkill(path, name, description string) *MarkdownSkill {
	return &MarkdownSkill{
		path:        path,
		name:        name,
		description: description,
	}
}

func (s *MarkdownSkill) Name() string {
	return s.name
}

func (s *MarkdownSkill) Description() string {
	return s.description
}

// Path returns the file path of the skill markdown.
func (s *MarkdownSkill) Path() string {
	return s.path
}

// SkillLoader loads skills from markdown files.
type SkillLoader struct {
	skillsDir string
}

// NewSkillLoader creates a new skill loader.
func NewSkillLoader(skillsDir string) *SkillLoader {
	return &SkillLoader{skillsDir: skillsDir}
}

// LoadSkill loads a single skill from a markdown file.
func (l *SkillLoader) LoadSkill(path string) (*MarkdownSkill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	return ParseSkillMarkdown(string(content), path)
}

// LoadAllSkills loads all skills from the skills directory.
func (l *SkillLoader) LoadAllSkills() ([]*MarkdownSkill, error) {
	if l.skillsDir == "" {
		return nil, nil
	}

	var skills []*MarkdownSkill

	err := filepath.Walk(l.skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for SKILL.md files or .md files in subdirectories
		if !info.IsDir() && (info.Name() == "SKILL.md") {
			skill, err := l.LoadSkill(path)
			if err != nil {
				slog.Warn("Failed to load skill", "path", path, "error", err)
				return nil
			}
			skills = append(skills, skill)
		}

		return nil
	})
	if os.IsNotExist(err) {
		// No skills directory is not an error - just means no skills to load
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to walk skills directory: %w", err)
	}

	return skills, nil
}

// skillMetadata represents the YAML frontmatter in a skill markdown file.
// Aligned with https://agentskills.io/specification
type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ParseSkillMarkdown parses a markdown skill file with YAML frontmatter.
func ParseSkillMarkdown(content string, path string) (*MarkdownSkill, error) {
	// Split frontmatter and content
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid skill format: missing YAML frontmatter")
	}

	// Parse YAML frontmatter
	var data skillMetadata
	if err := yaml.Unmarshal([]byte(parts[1]), &data); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	return NewMarkdownSkill(path, data.Name, data.Description), nil
}
