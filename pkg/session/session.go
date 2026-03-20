package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

// SessionRequest captures a normalized chat request for LLM providers.
type SessionRequest struct {
	SystemPrompt string
	Prompt       llm.Message
}

// Message is an alias for llm.Message to avoid import cycles with the llm package.
type Message = llm.Message

const RoleUser = llm.RoleUser
const RoleAssistant = llm.RoleAssistant
const RoleSystem = llm.RoleSystem

// minRecentMessages is the minimum number of recent messages to keep
// during compression, ensuring at least one user-assistant exchange
// remains in full for context continuity.
const minRecentMessages = 2

// Session tracks conversation state across multiple LLM completions.
type Session struct {
	llm        llm.LLM
	baseDir    string
	transcript []llm.Message
	saveFile   string
	savedCount int // Number of messages already persisted to disk
}

type opt func(*Session)

// WithTranscript initializes the session with a seed transcript. This can be used to provide context or examples for the conversation. The seed transcript is preserved and can be reset to with the Reset() method.
func WithTranscript(transcript []llm.Message) opt {
	return func(s *Session) {
		s.transcript = append([]llm.Message(nil), transcript...)
	}
}

// WithSave enables automatic session persistence after each interaction.
// The session will be saved to the specified filename in the session directory.
func WithSave(filename string) opt {
	return func(s *Session) {
		s.saveFile = filename
	}
}

// WithBaseDir sets the base directory for session file storage. If not set, the current working directory is used.
func WithBaseDir(dir string) opt {
	return func(s *Session) {
		s.baseDir = dir
	}
}

// NewSession creates a new Session bound to the provided LLM.
func NewSession(l llm.LLM, opts ...opt) *Session {
	s := &Session{
		llm: l,
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.saveFile == "" {
		s.saveFile = fmt.Sprintf("unnamed-session-%s.ndjson", time.Now().UTC().Format("060102150405"))
	}
	return s
}

// Complete sends the prompt through the underlying LLM and records the exchange.
func (s *Session) Complete(ctx context.Context, req SessionRequest) (llm.Message, error) {
	if s.llm == nil {
		return llm.Message{}, errors.New("llm provider is required")
	}

	systemPrompt := req.SystemPrompt
	prompt := req.Prompt
	if prompt.Role == "" {
		prompt.Role = llm.RoleUser
	}
	if prompt.Timestamp.IsZero() {
		prompt.Timestamp = time.Now()
	}

	history := s.transcript
	s.transcript = append(s.transcript, prompt)

	if err := s.Save(s.saveFile); err != nil {
		slog.Error("Failed to auto-save session", "error", err)
	}

	resp, err := s.llm.Complete(ctx, llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Transcript:   history,
		Prompt:       prompt,
	})
	if err != nil {
		return llm.Message{}, err
	}

	s.transcript = append(s.transcript, resp)

	if err := s.Save(s.saveFile); err != nil {
		slog.Error("Failed to auto-save session", "error", err)
	}

	return resp, nil
}

// PrepareCompress validates compression parameters and builds the text to
// summarize from the older messages. It returns the text to feed to the
// summarization LLM, the validated keepRecent count, and the total message
// count at the time of preparation.
func (s *Session) PrepareCompress(keepRecent int) (textToSummarize string, validatedKeep int, originalSize int, err error) {
	currentCount := len(s.transcript)
	if currentCount <= minRecentMessages {
		return "", 0, 0, fmt.Errorf("transcript too short to compress (minimum %d messages needed)", minRecentMessages)
	}

	// Determine how many recent messages to keep
	var keep int
	if keepRecent > 0 {
		keep = max(keepRecent, minRecentMessages)
		if keep >= currentCount {
			return "", 0, 0, fmt.Errorf("keep_recent (%d) must be less than current transcript size (%d)", keep, currentCount)
		}
	} else {
		// Default: keep half of current messages, minimum of 2
		keep = max(currentCount/2, minRecentMessages)
	}

	compressEnd := currentCount - keep
	toCompress := s.transcript[:compressEnd]

	// Build the conversation text for summarization
	var sb strings.Builder
	for _, msg := range toCompress {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}

	return sb.String(), keep, currentCount, nil
}

// ApplyCompression replaces the transcript with a summary message followed by
// recent messages. originalSize is the transcript size when PrepareCompress was
// called; any messages added after that point are preserved.
func (s *Session) ApplyCompression(summary string, keepRecent int, originalSize int) (string, error) {
	// Validate that the transcript hasn't shrunk unexpectedly
	if originalSize > len(s.transcript) {
		return "", fmt.Errorf("transcript has changed since compression started (was %d, now %d)", originalSize, len(s.transcript))
	}

	recentFromOriginal := s.transcript[originalSize-keepRecent : originalSize]
	newMessages := s.transcript[originalSize:]

	// Persist the full history before archiving it
	if err := s.Save(s.saveFile); err != nil {
		return "", fmt.Errorf("failed to save session before compression: %w", err)
	}
	archivePath, err := s.archiveSessionFile(s.saveFile)
	if err != nil {
		return "", fmt.Errorf("failed to archive session history: %w", err)
	}

	summaryMsg := llm.Message{
		Role:      llm.RoleAssistant,
		Content:   summary,
		Timestamp: time.Now(),
	}

	newTranscript := make([]llm.Message, 0, 1+len(recentFromOriginal)+len(newMessages))
	newTranscript = append(newTranscript, summaryMsg)
	newTranscript = append(newTranscript, recentFromOriginal...)
	newTranscript = append(newTranscript, newMessages...)

	s.transcript = newTranscript

	s.savedCount = 0

	if err := s.Save(s.saveFile); err != nil {
		slog.Error("Failed to auto-save session after compression", "error", err)
	}

	return archivePath, nil
}

// LLM returns the session's LLM provider.
func (s *Session) LLM() llm.LLM {
	return s.llm
}

// BaseDir returns the session's base directory.
func (s *Session) BaseDir() string {
	return s.baseDir
}

// Size returns the number of messages in the current transcript.
func (s *Session) Size() int {
	return len(s.transcript)
}

// Transcript returns the current conversation history.
func (s *Session) Transcript() []llm.Message {
	return append([]llm.Message(nil), s.transcript...)
}

// Reset clears the conversation history, keeping the initial seed transcript.
func (s *Session) Reset() {
	s.transcript = []llm.Message(nil)
}

func sanitizeSessionFilename(filename string) (string, error) {
	cleanName := filepath.Base(filename)
	if cleanName != filename || cleanName == "" || cleanName == "." {
		return "", fmt.Errorf("invalid session filename: %q", filename)
	}

	if !strings.HasSuffix(cleanName, ".ndjson") {
		cleanName += ".ndjson"
	}

	return cleanName, nil
}

func (s *Session) sessionFilePath(filename string) (string, error) {
	cleanName, err := sanitizeSessionFilename(filename)
	if err != nil {
		return "", err
	}

	sessionDir := filepath.Join(s.baseDir, "session")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	return filepath.Join(sessionDir, cleanName), nil
}

func (s *Session) archiveSessionFile(filename string) (string, error) {
	filePath, err := s.sessionFilePath(filename)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("session file does not exist: %s", filePath)
		}
		return "", fmt.Errorf("failed to stat session file: %w", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(filePath), ".ndjson")
	archivedName := fmt.Sprintf("%s-%s.ndjson", baseName, time.Now().UTC().Format("060102150405"))
	archivedPath := filepath.Join(filepath.Dir(filePath), archivedName)

	if err := os.Rename(filePath, archivedPath); err != nil {
		return "", fmt.Errorf("failed to archive session file: %w", err)
	}

	return archivedPath, nil
}

// Save persists the session to a file in the session directory.
// Only new messages (not yet saved) are appended to the file, making it efficient for auto-save.
// If savedCount is 0 or invalid, the entire file is rewritten (e.g., after compression).
func (s *Session) Save(filename string) error {
	filePath, err := s.sessionFilePath(filename)
	if err != nil {
		return err
	}

	// Determine if we need to rewrite or append
	needsRewrite := s.savedCount == 0 || s.savedCount > len(s.transcript)

	var messagesToSave []llm.Message
	var openFlags int

	if needsRewrite {
		// Rewrite entire file
		messagesToSave = s.transcript
		openFlags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	} else {
		// Append only new messages
		messagesToSave = s.transcript[s.savedCount:]
		if len(messagesToSave) == 0 {
			// Nothing new to save
			return nil
		}
		openFlags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}

	// Open file
	file, err := os.OpenFile(filePath, openFlags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	// Write messages as separate JSON lines
	for _, msg := range messagesToSave {
		if err := encoder.Encode(msg); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	// Update the count of saved messages
	s.savedCount = len(s.transcript)

	return nil
}

// Load restores the session from a file in the session directory.
func (s *Session) Load(filename string) error {
	// Ensure filename has .ndjson extension
	if !strings.HasSuffix(filename, ".ndjson") {
		filename = filename + ".ndjson"
	}

	// Build the full path
	sessionDir := filepath.Join(s.baseDir, "session")
	filePath := filepath.Join(sessionDir, filename)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	// Read messages line by line
	decoder := json.NewDecoder(file)
	var messages []llm.Message

	for decoder.More() {
		var msg llm.Message
		if err := decoder.Decode(&msg); err != nil {
			return fmt.Errorf("failed to decode message: %w", err)
		}
		messages = append(messages, msg)
	}

	// Restore the session state
	// Treat all loaded messages as the current transcript and as the new base.
	// This preserves Reset/WithTranscript semantics and keeps compression
	// from removing the loaded seed messages.
	s.transcript = messages
	// Mark all loaded messages as already saved
	s.savedCount = len(messages)

	return nil
}
