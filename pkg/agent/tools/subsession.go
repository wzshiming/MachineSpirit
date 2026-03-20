package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// subSessionInfo tracks the state of a running or completed sub-session.
type subSessionInfo struct {
	Name      string    `json:"name"`
	Task      string    `json:"task"`
	Status    string    `json:"status"` // "running", "completed", "failed"
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// SubSessionTool allows the agent to spawn sub-sessions that run tasks
// in the background and report results back through the main session's
// input queue.
type SubSessionTool struct {
	llmProvider llm.LLM
	pm          *persistence.PersistenceManager
	mainSession *session.Session
	buildTools  func() []agent.Tool

	mu          sync.Mutex
	subSessions map[string]*subSessionInfo
}

// NewSubSessionTool creates a new SubSessionTool.
// buildTools is a factory function that returns the set of tools available
// to each sub-session agent (typically a subset of the main agent's tools).
func NewSubSessionTool(
	llmProvider llm.LLM,
	pm *persistence.PersistenceManager,
	mainSession *session.Session,
	buildTools func() []agent.Tool,
) *SubSessionTool {
	return &SubSessionTool{
		llmProvider: llmProvider,
		pm:          pm,
		mainSession: mainSession,
		buildTools:  buildTools,
		subSessions: make(map[string]*subSessionInfo),
	}
}

func (t *SubSessionTool) Name() string {
	return "sub_session"
}

func (t *SubSessionTool) Description() string {
	return `Delegate a task to an independent sub-session that runs in the background.
Use this tool when you need to run independent tasks in parallel, handle long-running operations without blocking the conversation, or break a complex request into sub-tasks that can be worked on concurrently.
Results are automatically reported back when each sub-session completes.`
}

func (t *SubSessionTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "action", Type: "string", Required: true, Description: "The action to perform: 'start' to start a new sub-session, 'list' to list all sub-sessions."},
		{Name: "name", Type: "string", Required: false, Description: "Unique name for the sub-session (required for 'start' action)."},
		{Name: "task", Type: "string", Required: false, Description: "Description of the task to execute in the sub-session (required for 'start' action)."},
	}
}

func (t *SubSessionTool) Enabled() bool {
	return t.llmProvider != nil && t.mainSession != nil
}

func (t *SubSessionTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Action string `json:"action"`
		Name   string `json:"name"`
		Task   string `json:"task"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	switch params.Action {
	case "start":
		return t.startSubSession(ctx, params.Name, params.Task)
	case "list":
		return t.listSubSessions()
	default:
		return nil, fmt.Errorf("unknown action %q, use 'start' or 'list'", params.Action)
	}
}

func (t *SubSessionTool) startSubSession(ctx context.Context, name, task string) (json.RawMessage, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if task == "" {
		return nil, fmt.Errorf("task is required")
	}

	t.mu.Lock()
	if existing, ok := t.subSessions[name]; ok && existing.Status == "running" {
		t.mu.Unlock()
		return nil, fmt.Errorf("sub-session %q is already running", name)
	}

	info := &subSessionInfo{
		Name:      name,
		Task:      task,
		Status:    "running",
		StartTime: time.Now(),
	}
	t.subSessions[name] = info
	t.mu.Unlock()

	// Launch the sub-session in a background goroutine.
	go t.runSubSession(name, task)

	result, err := json.Marshal(map[string]any{
		"status":  "started",
		"name":    name,
		"task":    task,
		"message": fmt.Sprintf("Sub-session %q started. Results will be sent to the main session's input queue when complete.", name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}

func (t *SubSessionTool) runSubSession(name, task string) {
	// Use a fresh background context so the sub-session is not cancelled
	// when the parent tool_call context ends (the caller returns immediately
	// after launching the goroutine).
	ctx := context.Background()

	// Create a dedicated session for the sub-agent.
	saveFile := fmt.Sprintf("sub-session-%s-%s.ndjson", name, time.Now().UTC().Format("060102150405"))
	sess := session.NewSession(t.llmProvider,
		session.WithBaseDir(t.pm.GetBaseDir()),
		session.WithSave(saveFile),
	)

	// Build tools for the sub-session (typically a subset without sub_session itself).
	subTools := t.buildTools()

	ag, err := agent.NewAgent(
		sess,
		agent.WithPersistenceManager(t.pm),
		agent.WithTools(subTools...),
		agent.WithSkills(skills.NewSkills()),
		agent.WithMaxRetries(3),
	)
	if err != nil {
		t.finishSubSession(name, "", fmt.Sprintf("failed to create sub-agent: %v", err))
		return
	}

	var result bytes.Buffer
	err = ag.Execute(ctx, task, &result)
	if err != nil {
		t.finishSubSession(name, "", fmt.Sprintf("sub-session execution failed: %v", err))
		return
	}

	t.finishSubSession(name, result.String(), "")
}

func (t *SubSessionTool) finishSubSession(name, result, errMsg string) {
	t.mu.Lock()
	info, ok := t.subSessions[name]
	if ok {
		info.EndTime = time.Now()
		if errMsg != "" {
			info.Status = "failed"
			info.Error = errMsg
		} else {
			info.Status = "completed"
			info.Result = result
		}
	}
	t.mu.Unlock()

	// Send the result back through the main session's input queue.
	var content string
	if errMsg != "" {
		content = fmt.Sprintf("[Sub-session %q failed]: %s", name, errMsg)
		slog.Warn("Sub-session failed", "name", name, "error", errMsg)
	} else {
		content = fmt.Sprintf("[Sub-session %q completed]: %s", name, result)
		slog.Info("Sub-session completed", "name", name)
	}

	t.mainSession.AddInput(llm.Message{
		Role:      llm.RoleUser,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (t *SubSessionTool) listSubSessions() (json.RawMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	sessions := make([]map[string]any, 0, len(t.subSessions))
	for _, info := range t.subSessions {
		entry := map[string]any{
			"name":       info.Name,
			"task":       info.Task,
			"status":     info.Status,
			"start_time": info.StartTime.Format(time.RFC3339),
		}
		if !info.EndTime.IsZero() {
			entry["end_time"] = info.EndTime.Format(time.RFC3339)
			entry["duration"] = info.EndTime.Sub(info.StartTime).String()
		}
		if info.Result != "" {
			entry["result"] = info.Result
		}
		if info.Error != "" {
			entry["error"] = info.Error
		}
		sessions = append(sessions, entry)
	}

	result, err := json.Marshal(map[string]any{
		"sub_sessions": sessions,
		"total":        len(sessions),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
