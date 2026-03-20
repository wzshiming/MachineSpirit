package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

const (
	// minAllowedKeepRecent is the minimum allowed value for the keep_recent
	// parameter, ensuring at least one user-assistant exchange is preserved.
	minAllowedKeepRecent   = 2
	defaultKeepRecent      = 10
	compressToolThreshold  = 10
)

// CompressTool allows the agent to compress the conversation transcript.
type CompressTool struct {
	session *session.Session
}

// NewCompressTool creates a new Compress tool.
func NewCompressTool(sess *session.Session) *CompressTool {
	return &CompressTool{
		session: sess,
	}
}

func (t *CompressTool) Name() string {
	return "compress_transcript"
}

func (t *CompressTool) Description() string {
	currentSize := t.session.Size()
	return fmt.Sprintf("Compress the conversation transcript by summarizing older messages into a concise summary. Use this when the transcript is growing large to free up context. Current transcript size: %d messages.", currentSize)
}

func (t *CompressTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "keep_recent", Type: "int", Required: false, Description: fmt.Sprintf("Number of recent messages to keep uncompressed. Defaults to %d. Must be greater than %d if provided.", defaultKeepRecent, minAllowedKeepRecent)},
		{Name: "system_prompt", Type: "string", Required: false, Description: "The prompt used to instruct the LLM how to summarize the compressed messages. A sensible default is used if omitted."},
	}
}

func (t *CompressTool) Enabled() bool {
	return t.session != nil && t.session.Size() > compressToolThreshold
}

func (t *CompressTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		KeepRecent   int    `json:"keep_recent"`
		SystemPrompt string `json:"system_prompt"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Apply defaults for optional parameters
	if params.KeepRecent == 0 {
		params.KeepRecent = defaultKeepRecent
	}
	if params.KeepRecent <= minAllowedKeepRecent {
		return nil, fmt.Errorf("keep_recent must be greater than %d", minAllowedKeepRecent)
	}

	if params.SystemPrompt == "" {
		params.SystemPrompt = agent.DefaultCompressSystemPrompt
	}

	beforeCount := t.session.Size()

	// Perform compression with specified keep_recent value
	archivePath, err := t.session.CompressTranscript(ctx, params.KeepRecent, params.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	afterCount := t.session.Size()

	result, err := json.Marshal(map[string]any{
		"status":              "success",
		"messages_before":     beforeCount,
		"messages_after":      afterCount,
		"messages_compressed": beforeCount - afterCount,
		"archive_path":        archivePath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
