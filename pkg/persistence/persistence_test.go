package persistence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wzshiming/MachineSpirit/pkg/persistence/i18n"
)

func TestLocaleSpecificFileLoading(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create default SOUL.md
	defaultContent := "# Default Soul\nThis is the default personality."
	err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte(defaultContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create default SOUL.md: %v", err)
	}

	// Create Chinese localized SOUL.zh.md
	zhContent := "# 中文灵魂\n这是中文版本的人格设定。"
	err = os.WriteFile(filepath.Join(tmpDir, "SOUL.zh.md"), []byte(zhContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create SOUL.zh.md: %v", err)
	}

	pm, err := NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Test 1: No locale set - should use default file
	path := pm.getFilePath(i18n.FileSoul)
	if !strings.HasSuffix(path, "SOUL.md") {
		t.Errorf("Expected default SOUL.md, got %s", path)
	}
	content, _ := os.ReadFile(path)
	if string(content) != defaultContent {
		t.Errorf("Expected default content, got %s", string(content))
	}

	// Test 2: Set Chinese locale - should use SOUL.zh.md
	if err := pm.SetLocale("zh"); err != nil {
		t.Fatalf("Failed to set locale to zh: %v", err)
	}
	if pm.GetLocale() != "zh" {
		t.Errorf("Expected locale 'zh', got '%s'", pm.GetLocale())
	}
	path = pm.getFilePath(i18n.FileSoul)
	if !strings.HasSuffix(path, "SOUL.zh.md") {
		t.Errorf("Expected SOUL.zh.md, got %s", path)
	}
	content, _ = os.ReadFile(path)
	if string(content) != zhContent {
		t.Errorf("Expected Chinese content, got %s", string(content))
	}

	// Test 3: Set English locale - should fallback to default (no .en.md file)
	if err := pm.SetLocale("en"); err != nil {
		t.Fatalf("Failed to set locale to en: %v", err)
	}
	path = pm.getFilePath(i18n.FileSoul)
	if !strings.HasSuffix(path, "SOUL.md") {
		t.Errorf("Expected fallback to SOUL.md for en locale, got %s", path)
	}
	content, _ = os.ReadFile(path)
	if string(content) != defaultContent {
		t.Errorf("Expected default content for en locale, got %s", string(content))
	}

	// Test 4: Set unsupported locale - should error
	err = pm.SetLocale("es")
	if err == nil {
		t.Errorf("Expected error when setting unsupported locale 'es', but got none")
	}
}

func TestBuildSystemPromptWithLocale(t *testing.T) {
	tmpDir := t.TempDir()

	// Create default files
	defaultSoul := "You are a helpful assistant."
	err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte(defaultSoul), 0644)
	if err != nil {
		t.Fatalf("Failed to create SOUL.md: %v", err)
	}

	// Create localized files
	zhSoul := "你是一个有帮助的助手。"
	err = os.WriteFile(filepath.Join(tmpDir, "SOUL.zh.md"), []byte(zhSoul), 0644)
	if err != nil {
		t.Fatalf("Failed to create SOUL.zh.md: %v", err)
	}

	pm, err := NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Test 1: Default locale
	prompt := pm.BuildSystemPrompt("")
	if !strings.Contains(prompt, defaultSoul) {
		t.Errorf("Expected default prompt to contain '%s', got:\n%s", defaultSoul, prompt)
	}
	if strings.Contains(prompt, zhSoul) {
		t.Errorf("Expected default prompt NOT to contain Chinese text, got:\n%s", prompt)
	}

	// Test 2: Chinese locale
	if err := pm.SetLocale("zh"); err != nil {
		t.Fatalf("Failed to set locale to zh: %v", err)
	}
	prompt = pm.BuildSystemPrompt("")
	if !strings.Contains(prompt, zhSoul) {
		t.Errorf("Expected Chinese prompt to contain '%s', got:\n%s", zhSoul, prompt)
	}
	if strings.Contains(prompt, defaultSoul) {
		t.Errorf("Expected Chinese prompt NOT to contain English text, got:\n%s", prompt)
	}

	// Test 3: Verify timestamp is included regardless of locale
	if !strings.Contains(prompt, "Current time") {
		t.Errorf("Expected prompt to contain timestamp, got:\n%s", prompt)
	}
}

func TestMultipleLocalizedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple localized files
	files := map[string]string{
		"SOUL.md":      "Default soul",
		"SOUL.zh.md":   "中文灵魂",
		"USER.md":      "Default user",
		"USER.zh.md":   "中文用户",
		"MEMORY.md":    "Default memory",
		"MEMORY.zh.md": "中文记忆",
	}

	for filename, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", filename, err)
		}
	}

	pm, err := NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	// Test with Chinese locale
	if err := pm.SetLocale("zh"); err != nil {
		t.Fatalf("Failed to set locale to zh: %v", err)
	}
	prompt := pm.BuildSystemPrompt("")

	// Verify all Chinese files are loaded
	if !strings.Contains(prompt, "中文灵魂") {
		t.Errorf("Expected prompt to contain Chinese soul content")
	}
	if !strings.Contains(prompt, "中文用户") {
		t.Errorf("Expected prompt to contain Chinese user content")
	}
	if !strings.Contains(prompt, "中文记忆") {
		t.Errorf("Expected prompt to contain Chinese memory content")
	}

	// Verify English files are NOT loaded
	if strings.Contains(prompt, "Default soul") {
		t.Errorf("Expected prompt NOT to contain default soul content")
	}
	if strings.Contains(prompt, "Default user") {
		t.Errorf("Expected prompt NOT to contain default user content")
	}
	if strings.Contains(prompt, "Default memory") {
		t.Errorf("Expected prompt NOT to contain default memory content")
	}
}

func TestPartialLocalization(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only some localized files (mixed scenario)
	files := map[string]string{
		"SOUL.md":    "Default soul",
		"SOUL.zh.md": "中文灵魂",
		"USER.md":    "Default user",
		// No USER.zh.md - should fallback
		"MEMORY.md": "Default memory",
		// No MEMORY.zh.md - should fallback
	}

	for filename, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", filename, err)
		}
	}

	pm, err := NewPersistenceManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	if err := pm.SetLocale("zh"); err != nil {
		t.Fatalf("Failed to set locale to zh: %v", err)
	}
	prompt := pm.BuildSystemPrompt("")

	// SOUL should be in Chinese
	if !strings.Contains(prompt, "中文灵魂") {
		t.Errorf("Expected prompt to contain Chinese soul content")
	}
	if strings.Contains(prompt, "Default soul") {
		t.Errorf("Expected prompt NOT to contain default soul (has Chinese version)")
	}

	// USER and MEMORY should fallback to English
	if !strings.Contains(prompt, "Default user") {
		t.Errorf("Expected prompt to contain default user content (no Chinese version)")
	}
	if !strings.Contains(prompt, "Default memory") {
		t.Errorf("Expected prompt to contain default memory content (no Chinese version)")
	}
}
