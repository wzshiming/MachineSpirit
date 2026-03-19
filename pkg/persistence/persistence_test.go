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
