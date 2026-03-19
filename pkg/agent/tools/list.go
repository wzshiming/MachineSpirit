package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ListTool allows the agent to list directory contents.
type ListTool struct {
}

// NewListTool creates a new List tool.
func NewListTool() *ListTool {
	return &ListTool{}
}

func (t *ListTool) Name() string {
	return "list"
}

func (t *ListTool) Description() string {
	return "List contents of a directory with details. Set 'all' to include hidden files. {\"path\": \"/path/to/directory\", \"all\": false}."
}

func (t *ListTool) Enabled() bool {
	return true
}

type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

func (t *ListTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path string `json:"path"`
		All  bool   `json:"all"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Default to current directory if no path provided
	if params.Path == "" {
		params.Path = "."
	}

	// Check if path exists and is a directory
	info, err := os.Stat(params.Path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", params.Path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", params.Path)
	}

	// Read directory entries
	entries, err := os.ReadDir(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Collect file information
	var files []FileInfo
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files if 'all' is false
		if !params.All && len(name) > 0 && name[0] == '.' {
			continue
		}

		fullPath := filepath.Join(params.Path, name)
		fileInfo, err := entry.Info()
		if err != nil {
			// Skip files we can't stat
			continue
		}

		files = append(files, FileInfo{
			Name:    name,
			Path:    fullPath,
			Size:    fileInfo.Size(),
			Mode:    fileInfo.Mode().String(),
			ModTime: fileInfo.ModTime().Format(time.RFC3339),
			IsDir:   entry.IsDir(),
		})
	}

	result, err := json.Marshal(map[string]any{
		"path":   params.Path,
		"count":  len(files),
		"files":  files,
		"status": "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
