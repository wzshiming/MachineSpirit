package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

const (
	// minAllowedKeepRecent is the minimum allowed value for the keep_recent
	// parameter, ensuring at least one user-assistant exchange is preserved.
	minAllowedKeepRecent  = 2
	defaultKeepRecent     = 10
	compressToolThreshold = 10
)

// CompressTool allows the agent to compress the conversation transcript.
// Compression runs in a background goroutine using a separate sub-session
// for the LLM summarization call, so the main thread is not blocked.
// Results are reported back through the addInput callback.
type CompressTool struct {
	session     *session.Session
	llmProvider llm.LLM
}

// NewCompressTool creates a new Compress tool.
// llmProvider is used to create the sub-session for summarization.
// addInput is a callback that enqueues a message into the parent agent's
// input queue when compression completes.
func NewCompressTool(sess *session.Session, llmProvider llm.LLM) *CompressTool {
	return &CompressTool{
		session:     sess,
		llmProvider: llmProvider,
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
	return t.session != nil && t.llmProvider != nil && t.session.Size() > compressToolThreshold
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

	// Prepare compression data synchronously (fast, no LLM call).
	text, keepRecent, originalSize, err := t.session.PrepareCompress(params.KeepRecent)
	if err != nil {
		return nil, fmt.Errorf("compression preparation failed: %w", err)
	}

	// Launch the summarization in a background goroutine.
	go t.runCompression(text, keepRecent, originalSize, params.SystemPrompt)

	result, err := json.Marshal(map[string]any{
		"status":         "started",
		"messages_count": originalSize,
		"message":        "Compression started in a background sub-session. Results will be reported when complete.",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}

// runCompression performs the summarization in a background goroutine using a
// dedicated sub-session, then applies the result to the main session and
// reports back via addInput.
func (t *CompressTool) runCompression(text string, keepRecent, originalSize int, systemPrompt string) {
	ctx := context.Background()

	// Create a sub-session for the summarization LLM call.
	saveFile := fmt.Sprintf("compress-%s.ndjson", time.Now().UTC().Format("060102150405"))
	subSess := session.NewSession(t.llmProvider,
		session.WithBaseDir(t.session.BaseDir()),
		session.WithSave(saveFile),
	)

	summaryResp, err := subSess.Complete(ctx, session.SessionRequest{
		SystemPrompt: systemPrompt,
		Prompt: session.Message{
			Role:    session.RoleUser,
			Content: text,
		},
	})
	if err != nil {
		slog.Warn("Compression sub-session failed", "error", err)
		return
	}

	// Apply the compression result to the main session.
	archivePath, err := t.session.ApplyCompression(summaryResp.Content, keepRecent, originalSize)
	if err != nil {
		slog.Warn("Failed to apply compression", "error", err)
		return
	}

	afterCount := t.session.Size()
	slog.Info("Compression completed",
		"before", originalSize,
		"after", afterCount,
		"archive", archivePath,
	)
}
