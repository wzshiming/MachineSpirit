package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wzshiming/MachineSpirit/pkg/session"
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
	return fmt.Sprintf(`Compress the conversation transcript by summarizing older messages into a concise summary. Current transcript size: %d messages. {"keep_recent": 10, "system_prompt": "Summarize the following conversation concisely, preserving key information, decisions, and context that would be needed to continue the conversation. Output only the summary."} - Keep the 10 most recent messages, compress the rest`, currentSize)
}

func (t *CompressTool) Enabled() bool {
	return t.session != nil && t.session.Size() > minRecentMessages*2
}

const minRecentMessages = 2

func (t *CompressTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		KeepRecent   int    `json:"keep_recent"`
		SystemPrompt string `json:"system_prompt"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.KeepRecent <= 2 {
		return nil, fmt.Errorf("keep_recent must be greater than 2")
	}

	if params.SystemPrompt == "" {
		return nil, fmt.Errorf("system_prompt is required")
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
