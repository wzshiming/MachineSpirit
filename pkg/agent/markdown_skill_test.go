package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

func TestParseSkillMarkdown(t *testing.T) {
	content := `---
name: test_skill
description: A test skill for demonstration
license: MIT
tags:
  - test
  - example
memory:
  key1: value1
  key2: value2
---

# Test Skill

This is a test skill with instructions.

## Usage

Follow these steps...
`

	skill, err := ParseSkillMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name() != "test_skill" {
		t.Fatalf("expected name 'test_skill', got %s", skill.Name())
	}

	if skill.Description() != "A test skill for demonstration" {
		t.Fatalf("unexpected description: %s", skill.Description())
	}

	instructions := skill.Instructions()
	if instructions == "" {
		t.Fatal("instructions should not be empty")
	}

	if !skill.IsInstructionBased() {
		t.Fatal("should be instruction-based")
	}
}

func TestParseSkillMarkdownInvalidFormat(t *testing.T) {
	content := "No frontmatter here"

	_, err := ParseSkillMarkdown(content)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestMarkdownSkillExecute(t *testing.T) {
	skill := NewMarkdownSkill("test", "test skill", "instructions here", nil)

	ctx := context.Background()
	input := json.RawMessage(`{"query": "test"}`)

	output, err := skill.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Markdown skills return instructions when executed
	if output == "" {
		t.Fatal("output should not be empty")
	}

	if !contains(output, "instructions here") {
		t.Fatalf("output should contain instructions: %s", output)
	}
}

func TestSkillLoader(t *testing.T) {
	// Create a temporary directory with a test skill
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "test_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	skillContent := `---
name: temp_skill
description: Temporary test skill
license: MIT
---

# Temporary Skill

Test instructions.
`

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write skill file: %v", err)
	}

	// Test loading
	loader := NewSkillLoader(tempDir)
	skill, err := loader.LoadSkill(skillPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name() != "temp_skill" {
		t.Fatalf("expected name 'temp_skill', got %s", skill.Name())
	}
}

func TestSkillLoaderLoadAll(t *testing.T) {
	// Create a temporary directory with multiple skills
	tempDir := t.TempDir()

	// Skill 1
	skill1Dir := filepath.Join(tempDir, "skill1")
	if err := os.MkdirAll(skill1Dir, 0755); err != nil {
		t.Fatalf("failed to create skill1 dir: %v", err)
	}
	skill1Content := `---
name: skill1
description: First skill
license: MIT
---

# Skill 1
`
	if err := os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0644); err != nil {
		t.Fatalf("failed to write skill1 file: %v", err)
	}

	// Skill 2
	skill2Dir := filepath.Join(tempDir, "skill2")
	if err := os.MkdirAll(skill2Dir, 0755); err != nil {
		t.Fatalf("failed to create skill2 dir: %v", err)
	}
	skill2Content := `---
name: skill2
description: Second skill
license: MIT
---

# Skill 2
`
	if err := os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(skill2Content), 0644); err != nil {
		t.Fatalf("failed to write skill2 file: %v", err)
	}

	// Load all skills
	loader := NewSkillLoader(tempDir)
	skills, err := loader.LoadAllSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	// Verify both skills loaded
	names := make(map[string]bool)
	for _, skill := range skills {
		names[skill.Name()] = true
	}

	if !names["skill1"] || !names["skill2"] {
		t.Fatal("not all skills were loaded")
	}
}

func TestAgentLoadSkillsFromDirectory(t *testing.T) {
	// Create a temporary directory with a skill
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "test_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	skillContent := `---
name: loaded_skill
description: A skill loaded from directory
license: MIT
---

# Loaded Skill

Instructions for the agent.
`

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write skill file: %v", err)
	}

	// Create agent
	mockLLM := &mockLLM{responses: []string{"ok"}}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Load skills from directory
	if err := agent.LoadSkillsFromDirectory(tempDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify skill was loaded
	registry := agent.GetSkillRegistry()
	if !registry.Has("loaded_skill") {
		t.Fatal("skill should be loaded")
	}

	skill, err := registry.Get("loaded_skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name() != "loaded_skill" {
		t.Fatalf("unexpected skill name: %s", skill.Name())
	}
}

func TestAgentPromptWithInstructionSkills(t *testing.T) {
	mockLLM := &mockLLM{responses: []string{"response"}}
	session := llm.NewSession(mockLLM, llm.SessionConfig{})

	agent, err := NewAgent(Config{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Register an instruction-based skill
	skill := NewMarkdownSkill(
		"test_guide",
		"A test guide",
		"# Test Guide\n\nFollow these instructions:\n1. Step one\n2. Step two",
		nil,
	)

	if err := agent.RegisterSkill(skill); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build prompt - it should include the skill instructions
	prompt := agent.buildPrompt("test request", "")

	// Verify the prompt includes the expert skills section
	if !contains(prompt, "## Expert Skills") {
		t.Fatal("prompt should include Expert Skills section")
	}

	if !contains(prompt, "Follow these instructions") {
		t.Fatal("prompt should include skill instructions")
	}
}
