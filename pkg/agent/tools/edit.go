package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// EditTool allows the agent to edit files by replacing or inserting content.
type EditTool struct{}

// NewEditTool creates a new Edit tool.
func NewEditTool() *EditTool {
	return &EditTool{}
}

func (t *EditTool) Name() string {
	return "edit"
}

func (t *EditTool) Description() string {
	return "Edit a file by replacing a range of lines or inserting content after a specific line."
}

func (t *EditTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "path", Type: "string", Required: true, Description: "Absolute path to the file to edit."},
		{Name: "content", Type: "string", Required: true, Description: "The new content to insert or replace with."},
		{Name: "start", Type: "int", Required: false, Description: "Start line number (1-based). When used with end, defines the range to replace. When used alone, inserts content after this line."},
		{Name: "end", Type: "int", Required: false, Description: "End line number for replacement. Use -1 to replace from start to end of file. Omit to insert instead of replace."},
	}
}

func (t *EditTool) Enabled() bool {
	return true
}

type editParams struct {
	Path    string `json:"path"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Content string `json:"content"`
}

func (t *EditTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params editParams
	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	if params.Content == "" {
		return nil, fmt.Errorf("content parameter is required")
	}

	// Read existing file
	lines, err := readAllLines(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var newLines []string
	action := "insert"

	if params.End > 0 {
		// Replace lines from start to end
		action = "replace"
		// Lines before start
		for i := 0; i < len(lines) && i < params.Start-1; i++ {
			newLines = append(newLines, lines[i])
		}
		// Add new content
		newLines = append(newLines, strings.Split(params.Content, "\n")...)
		// Lines after end
		for i := params.End; i < len(lines); i++ {
			newLines = append(newLines, lines[i])
		}
	} else if params.End == -1 {
		// Replace from start to end of file
		action = "replace"
		// Add new content
		newLines = append(newLines, strings.Split(params.Content, "\n")...)
	} else if params.Start > 0 {
		// Insert after line start
		action = "insert"
		// Lines before insert position
		for i := 0; i < len(lines) && i < params.Start; i++ {
			newLines = append(newLines, lines[i])
		}
		// Add new content
		newLines = append(newLines, strings.Split(params.Content, "\n")...)
		// Lines after insert position
		for i := params.Start; i < len(lines); i++ {
			newLines = append(newLines, lines[i])
		}
	} else {
		// No start/end specified, treat as insert at beginning
		newLines = append(newLines, strings.Split(params.Content, "\n")...)
		newLines = append(newLines, lines...)
	}

	// Write back to file
	newContent := strings.Join(newLines, "\n")
	// Ensure trailing newline
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	err = os.WriteFile(params.Path, []byte(newContent), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"path":   params.Path,
		"status": "success",
		"action": action,
		"lines":  len(newLines),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}

// readAllLines reads all lines from a file
func readAllLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}
