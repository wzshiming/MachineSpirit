package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillLoader(t *testing.T) {
	// Create a temporary directory with a test skill
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "test_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	skillContent := `---
id: temp_skill
title: Temporary Skill
description: Temporary test skill
author: Test
version: 1.0.0
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
id: skill1
title: Skill 1
description: First skill
author: Test
version: 1.0.0
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
id: skill2
title: Skill 2
description: Second skill
author: Test
version: 1.0.0
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
