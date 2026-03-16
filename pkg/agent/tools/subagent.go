package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// SubAgentTool allows the master agent to delegate specific tasks to a subagent.
// The master agent can observe the subagent's execution and the system always
// waits for human input before executing sensitive operations.
type SubAgentTool struct {
	llm            llm.LLM
	pm             *persistence.PersistenceManager
	approvalFunc   ApprovalFunc
	observerFunc   ObserverFunc
	availableTools []agent.Tool
}

// ApprovalFunc is called to request human approval for subagent actions.
// It returns true if the action is approved, false otherwise.
type ApprovalFunc func(ctx context.Context, action string) (bool, error)

// ObserverFunc is called to notify the master agent of subagent progress.
type ObserverFunc func(message string)

// SubAgentInput represents the input for creating and running a subagent.
type SubAgentInput struct {
	Task        string   `json:"task"`        // The specific task to delegate to the subagent
	Description string   `json:"description"` // Optional description of what the subagent should accomplish
	Tools       []string `json:"tools"`       // List of tool names the subagent can use (empty means all available)
}

// SubAgentOutput represents the result of subagent execution.
type SubAgentOutput struct {
	Status    string   `json:"status"`              // "success", "failed", or "cancelled"
	Result    string   `json:"result"`              // The final result from the subagent
	Actions   []string `json:"actions"`             // List of actions the subagent performed
	Error     string   `json:"error,omitempty"`     // Error message if status is "failed"
	Cancelled string   `json:"cancelled,omitempty"` // Reason if status is "cancelled"
}

// NewSubAgentTool creates a new subagent tool with the given configuration.
func NewSubAgentTool(
	llm llm.LLM,
	pm *persistence.PersistenceManager,
	availableTools []agent.Tool,
	approvalFunc ApprovalFunc,
	observerFunc ObserverFunc,
) *SubAgentTool {
	if approvalFunc == nil {
		// Default to always approve
		approvalFunc = func(ctx context.Context, action string) (bool, error) {
			return true, nil
		}
	}
	if observerFunc == nil {
		// Default to no-op observer
		observerFunc = func(message string) {}
	}
	return &SubAgentTool{
		llm:            llm,
		pm:             pm,
		approvalFunc:   approvalFunc,
		observerFunc:   observerFunc,
		availableTools: availableTools,
	}
}

// Name returns the tool name.
func (t *SubAgentTool) Name() string {
	return "subagent"
}

// Description returns a description of what this tool does.
func (t *SubAgentTool) Description() string {
	return "Delegate a specific task to a subagent. The subagent can use tools to accomplish the task, and its actions are observable. Use this when you need to focus on a specific subtask independently. Human approval is required for sensitive operations."
}

// Execute runs the subagent with the given task.
func (t *SubAgentTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params SubAgentInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid subagent input: %w", err)
	}

	if params.Task == "" {
		return nil, fmt.Errorf("task is required")
	}

	// Notify observer that subagent is starting
	t.observerFunc(fmt.Sprintf("🤖 Starting subagent for task: %s", params.Task))

	// Request human approval to start the subagent
	approved, err := t.approvalFunc(ctx, fmt.Sprintf("Start subagent for task: %s", params.Task))
	if err != nil {
		return marshalOutput(SubAgentOutput{
			Status:    "failed",
			Error:     fmt.Sprintf("approval check failed: %v", err),
			Actions:   []string{},
		})
	}
	if !approved {
		return marshalOutput(SubAgentOutput{
			Status:    "cancelled",
			Cancelled: "User did not approve subagent execution",
			Actions:   []string{},
		})
	}

	// Filter tools based on input
	tools := t.filterTools(params.Tools)

	// Create a new session for the subagent with a custom system prompt
	subSession := session.NewSession(t.llm)

	// Create the subagent with observable tool wrapper
	actions := []string{}
	observableTools := make([]agent.Tool, 0, len(tools))
	for _, tool := range tools {
		observableTools = append(observableTools, &observableTool{
			tool:         tool,
			approvalFunc: t.approvalFunc,
			observerFunc: t.observerFunc,
			actions:      &actions,
		})
	}

	subAgent, err := agent.NewAgent(
		subSession,
		agent.WithPersistenceManager(t.pm),
		agent.WithTools(observableTools...),
		agent.WithMaxRetries(10),
	)
	if err != nil {
		return marshalOutput(SubAgentOutput{
			Status:  "failed",
			Error:   fmt.Sprintf("failed to create subagent: %v", err),
			Actions: actions,
		})
	}

	// Build the task prompt
	taskPrompt := t.buildTaskPrompt(params)

	// Execute the subagent
	t.observerFunc("📋 Subagent is processing the task...")
	result, err := subAgent.Execute(ctx, taskPrompt)
	if err != nil {
		return marshalOutput(SubAgentOutput{
			Status:  "failed",
			Error:   fmt.Sprintf("subagent execution failed: %v", err),
			Result:  result,
			Actions: actions,
		})
	}

	t.observerFunc("✅ Subagent completed the task")

	return marshalOutput(SubAgentOutput{
		Status:  "success",
		Result:  result,
		Actions: actions,
	})
}

// filterTools returns the subset of tools specified in the input, or all tools if none specified.
func (t *SubAgentTool) filterTools(toolNames []string) []agent.Tool {
	if len(toolNames) == 0 {
		return t.availableTools
	}

	toolMap := make(map[string]agent.Tool)
	for _, tool := range t.availableTools {
		toolMap[tool.Name()] = tool
	}

	filtered := make([]agent.Tool, 0, len(toolNames))
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// buildTaskPrompt constructs the prompt for the subagent.
func (t *SubAgentTool) buildTaskPrompt(params SubAgentInput) string {
	var sb strings.Builder

	sb.WriteString("You are a specialized subagent working on a specific task.\n\n")
	sb.WriteString(fmt.Sprintf("**Your Task**: %s\n\n", params.Task))

	if params.Description != "" {
		sb.WriteString(fmt.Sprintf("**Additional Context**: %s\n\n", params.Description))
	}

	sb.WriteString("**Instructions**:\n")
	sb.WriteString("1. Focus exclusively on the task assigned to you\n")
	sb.WriteString("2. Use the available tools to accomplish the task\n")
	sb.WriteString("3. Be concise and efficient in your approach\n")
	sb.WriteString("4. Provide a clear summary of what you accomplished\n")
	sb.WriteString("5. Note: Human approval is required for each tool invocation\n\n")
	sb.WriteString("Begin working on your task now.\n")

	return sb.String()
}

// marshalOutput is a helper to marshal the output to JSON.
func marshalOutput(output SubAgentOutput) (json.RawMessage, error) {
	data, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output: %w", err)
	}
	return data, nil
}

// observableTool wraps a tool to provide observation and approval capabilities.
type observableTool struct {
	tool         agent.Tool
	approvalFunc ApprovalFunc
	observerFunc ObserverFunc
	actions      *[]string
}

func (o *observableTool) Name() string {
	return o.tool.Name()
}

func (o *observableTool) Description() string {
	return o.tool.Description()
}

func (o *observableTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	// Notify observer of the pending action
	action := fmt.Sprintf("Tool: %s, Input: %s", o.tool.Name(), string(input))
	o.observerFunc(fmt.Sprintf("🔧 Subagent wants to use: %s", o.tool.Name()))

	// Request human approval
	approved, err := o.approvalFunc(ctx, action)
	if err != nil {
		return nil, fmt.Errorf("approval check failed: %w", err)
	}
	if !approved {
		return nil, fmt.Errorf("action was not approved by user")
	}

	// Execute the tool
	o.observerFunc(fmt.Sprintf("⚙️  Executing: %s", o.tool.Name()))
	result, err := o.tool.Execute(ctx, input)

	// Record the action
	*o.actions = append(*o.actions, fmt.Sprintf("%s: %s", o.tool.Name(), summarizeResult(result, err)))

	if err != nil {
		o.observerFunc(fmt.Sprintf("❌ Tool %s failed: %v", o.tool.Name(), err))
		return result, err
	}

	o.observerFunc(fmt.Sprintf("✓ Tool %s completed", o.tool.Name()))
	return result, nil
}

// summarizeResult provides a brief summary of the tool result for logging.
func summarizeResult(result json.RawMessage, err error) string {
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(result) > 100 {
		return string(result[:97]) + "..."
	}
	return string(result)
}
