package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MarkdownSkill represents an instruction-based skill loaded from a markdown file.
// This follows the Anthropic Skills pattern where skills are instructional guides
// rather than executable code.
type MarkdownSkill struct {
	name         string
	description  string
	instructions string // Full markdown content
	metadata     map[string]string
	license      string
}

// SkillMetadata represents the YAML frontmatter in a skill markdown file.
type SkillMetadata struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	License     string            `yaml:"license"`
	Tags        []string          `yaml:"tags"`
	Memory      map[string]string `yaml:"memory"` // Memory hints for the skill
}

// NewMarkdownSkill creates a skill from markdown content.
func NewMarkdownSkill(name, description, instructions string, metadata map[string]string) *MarkdownSkill {
	return &MarkdownSkill{
		name:         name,
		description:  description,
		instructions: instructions,
		metadata:     metadata,
	}
}

func (s *MarkdownSkill) Name() string {
	return s.name
}

func (s *MarkdownSkill) Description() string {
	return s.description
}

func (s *MarkdownSkill) DetailedDescription() string {
	// For markdown skills, the detailed description is the instructions
	return s.instructions
}

func (s *MarkdownSkill) ParametersSchema() map[string]interface{} {
	// Markdown skills don't have a formal parameter schema
	// They're instruction sets, not executable functions
	return map[string]interface{}{
		"type":        "object",
		"description": "This is an instruction-based skill. Follow the instructions provided.",
	}
}

func (s *MarkdownSkill) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	// Markdown skills don't execute - they provide instructions
	// Return the instructions so the agent knows how to proceed
	return fmt.Sprintf("This is an instruction-based skill. Follow these instructions:\n\n%s", s.instructions), nil
}

// Instructions returns the full markdown instructions.
func (s *MarkdownSkill) Instructions() string {
	return s.instructions
}

// Metadata returns the skill metadata.
func (s *MarkdownSkill) Metadata() map[string]string {
	return s.metadata
}

// IsInstructionBased returns true for markdown skills.
func (s *MarkdownSkill) IsInstructionBased() bool {
	return true
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

	return ParseSkillMarkdown(string(content))
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
		if !info.IsDir() && (info.Name() == "SKILL.md" || strings.HasSuffix(info.Name(), ".skill.md")) {
			skill, err := l.LoadSkill(path)
			if err != nil {
				// Log error but continue loading other skills
				fmt.Fprintf(os.Stderr, "Warning: failed to load skill from %s: %v\n", path, err)
				return nil
			}
			skills = append(skills, skill)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk skills directory: %w", err)
	}

	return skills, nil
}

// ParseSkillMarkdown parses a markdown skill file with YAML frontmatter.
func ParseSkillMarkdown(content string) (*MarkdownSkill, error) {
	// Split frontmatter and content
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid skill format: missing YAML frontmatter")
	}

	// Parse YAML frontmatter
	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(parts[1]), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Extract markdown content
	instructions := strings.TrimSpace(parts[2])

	// Convert metadata to map
	metadataMap := map[string]string{
		"license": metadata.License,
	}
	if len(metadata.Tags) > 0 {
		metadataMap["tags"] = strings.Join(metadata.Tags, ",")
	}
	for k, v := range metadata.Memory {
		metadataMap["memory_"+k] = v
	}

	return NewMarkdownSkill(metadata.Name, metadata.Description, instructions, metadataMap), nil
}

// InstructionSkill is a marker interface for instruction-based skills.
type InstructionSkill interface {
	Skill
	Instructions() string
	IsInstructionBased() bool
}
