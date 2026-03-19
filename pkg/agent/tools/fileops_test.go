package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveTool(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	err := os.WriteFile(sourcePath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	tool := NewMoveTool()
	ctx := context.Background()

	// Test move operation
	input, _ := json.Marshal(map[string]interface{}{
		"source":      sourcePath,
		"destination": destPath,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Verify source file no longer exists
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Error("Source file still exists after move")
	}

	// Verify destination file exists
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Destination file does not exist after move: %v", err)
	}
}

func TestTrashTool(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create test file
	err := os.WriteFile(filePath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewTrashTool()
	ctx := context.Background()

	// Test delete operation
	input, _ := json.Marshal(map[string]interface{}{
		"path":      filePath,
		"recursive": false,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Verify file no longer exists
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("File still exists after trash")
	}
}

func TestTrashToolRecursive(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")
	filePath := filepath.Join(dirPath, "test.txt")

	// Create directory with file
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	err = os.WriteFile(filePath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewTrashTool()
	ctx := context.Background()

	// Test recursive delete operation
	input, _ := json.Marshal(map[string]interface{}{
		"path":      dirPath,
		"recursive": true,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Verify directory no longer exists
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory still exists after recursive trash")
	}
}

func TestMkdirTool(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "newdir")

	tool := NewMkdirTool()
	ctx := context.Background()

	// Test mkdir operation
	input, _ := json.Marshal(map[string]interface{}{
		"path":    dirPath,
		"parents": false,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Verify directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Errorf("Directory does not exist after mkdir: %v", err)
	}
	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}
}

func TestMkdirToolWithParents(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "parent", "child", "grandchild")

	tool := NewMkdirTool()
	ctx := context.Background()

	// Test mkdir with parents operation
	input, _ := json.Marshal(map[string]interface{}{
		"path":    dirPath,
		"parents": true,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Verify directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Errorf("Directory does not exist after mkdir: %v", err)
	}
	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}
}

func TestListTool(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := NewListTool()
	ctx := context.Background()

	// Test list operation without hidden files
	input, _ := json.Marshal(map[string]interface{}{
		"path": tmpDir,
		"all":  false,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Should have 3 visible items (file1.txt, file2.txt, subdir)
	count := int(resultData["count"].(float64))
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestListToolWithHidden(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()

	// Create test files including hidden
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

	tool := NewListTool()
	ctx := context.Background()

	// Test list operation with hidden files
	input, _ := json.Marshal(map[string]interface{}{
		"path": tmpDir,
		"all":  true,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Should have 2 items (file1.txt and .hidden)
	count := int(resultData["count"].(float64))
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}
