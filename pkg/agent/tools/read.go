package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadTool allows the agent to read files.
type ReadTool struct {
}

// NewReadTool creates a new Read tool.
func NewReadTool() *ReadTool {
	return &ReadTool{}
}

func (t *ReadTool) Name() string {
	return "read"
}

func (t *ReadTool) Description() string {
	return "Read the content of a file. Use 'lines' to limit output (e.g., first 100 lines). Use 'start' and 'end' to read specific line ranges. {\"path\": \"/path/to/file\", \"lines\": 100, \"start\": 1, \"end\": 50}."
}

func (t *ReadTool) Enabled() bool {
	return true
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path  string `json:"path"`
		Lines int    `json:"lines"`
		Start int    `json:"start"`
		End   int    `json:"end"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("file parameter is required")
	}

	// Check if file exists
	info, err := os.Stat(params.Path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", params.Path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", params.Path)
	}

	file, err := os.Open(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	lineNum := 1
	totalLines := 0
	linesRead := 0

	// Determine if we should read with limits
	hasLineLimit := params.Lines > 0
	hasRange := params.Start > 0 || params.End > 0

	for scanner.Scan() {
		totalLines++

		// Check if we should include this line based on range
		shouldInclude := true
		if hasRange {
			if params.Start > 0 && lineNum < params.Start {
				shouldInclude = false
			}
			if params.End > 0 && lineNum > params.End {
				break
			}
		}

		if shouldInclude {
			if hasLineLimit && linesRead >= params.Lines {
				break
			}
			if linesRead > 0 {
				content.WriteString("\n")
			}
			content.WriteString(scanner.Text())
			linesRead++
		}

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"path":        params.Path,
		"content":     content.String(),
		"size":        info.Size(),
		"lines_read":  linesRead,
		"total_lines": totalLines,
		"truncated":   (hasLineLimit && totalLines > params.Lines) || (hasRange && params.End > 0 && totalLines > params.End),
		"status":      "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
