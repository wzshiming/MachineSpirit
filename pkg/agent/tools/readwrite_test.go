package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteToolAppend(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	tool := NewWriteTool()
	ctx := context.Background()

	// Write initial content
	input1, _ := json.Marshal(map[string]interface{}{
		"path":    filePath,
		"content": "First line\n",
		"append":  false,
	})

	result1, err := tool.Execute(ctx, input1)
	if err != nil {
		t.Errorf("First write failed: %v", err)
	}

	var resultData1 map[string]interface{}
	json.Unmarshal(result1, &resultData1)
	if resultData1["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData1["status"])
	}

	// Append more content
	input2, _ := json.Marshal(map[string]interface{}{
		"path":    filePath,
		"content": "Second line\n",
		"append":  true,
	})

	result2, err := tool.Execute(ctx, input2)
	if err != nil {
		t.Errorf("Append failed: %v", err)
	}

	var resultData2 map[string]interface{}
	json.Unmarshal(result2, &resultData2)
	if resultData2["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData2["status"])
	}

	// Verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	expected := "First line\nSecond line\n"
	if string(content) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, string(content))
	}
}

func TestWriteToolCreateParentDirs(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "parent", "child", "test.txt")

	tool := NewWriteTool()
	ctx := context.Background()

	// Write to nested path
	input, _ := json.Marshal(map[string]interface{}{
		"path":    filePath,
		"content": "test content",
		"append":  false,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Write with parent dir creation failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("File does not exist: %v", err)
	}
}

func TestReadToolWithLines(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create file with multiple lines
	lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}
	err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadTool()
	ctx := context.Background()

	// Read only first 3 lines
	input, _ := json.Marshal(map[string]interface{}{
		"path":  filePath,
		"lines": 3,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Read with lines limit failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	linesRead := int(resultData["lines_read"].(float64))
	if linesRead != 3 {
		t.Errorf("Expected 3 lines read, got %d", linesRead)
	}

	content := resultData["content"].(string)
	if content != "Line 1\nLine 2\nLine 3" {
		t.Errorf("Unexpected content: '%s'", content)
	}

	truncated := resultData["truncated"].(bool)
	if !truncated {
		t.Error("Expected truncated to be true")
	}
}

func TestReadToolWithRange(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create file with multiple lines
	lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}
	err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadTool()
	ctx := context.Background()

	// Read lines 2-4
	input, _ := json.Marshal(map[string]interface{}{
		"path":  filePath,
		"start": 2,
		"end":   4,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Read with range failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	content := resultData["content"].(string)
	if content != "Line 2\nLine 3\nLine 4" {
		t.Errorf("Unexpected content: '%s'", content)
	}
}

func TestReadToolFullFile(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create simple file
	expectedContent := "Hello World\nThis is a test"
	err := os.WriteFile(filePath, []byte(expectedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadTool()
	ctx := context.Background()

	// Read full file
	input, _ := json.Marshal(map[string]interface{}{
		"path": filePath,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}

	var resultData map[string]interface{}
	json.Unmarshal(result, &resultData)
	if resultData["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", resultData["status"])
	}

	content := resultData["content"].(string)
	if content != expectedContent {
		t.Errorf("Expected '%s', got '%s'", expectedContent, content)
	}

	truncated := resultData["truncated"].(bool)
	if truncated {
		t.Error("Expected truncated to be false")
	}
}
