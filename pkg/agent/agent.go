package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	jsonrepair "github.com/RealAlexandreAI/json-repair"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// Agent orchestrates multi-step reasoning with tool calling and memory.
type Agent struct {
	session    *session.Session
	tools      map[string]Tool
	skills     map[string]Skill
	maxRetries int
}

type opt func(*Agent)

// WithTools sets the tools available to the agent.
func WithTools(tools ...Tool) opt {
	return func(a *Agent) {
		for _, tool := range tools {
			a.tools[tool.Name()] = tool
		}
	}
}

// WithSkills sets the skills available to the agent.
func WithSkills(skills ...Skill) opt {
	return func(a *Agent) {
		for _, skill := range skills {
			a.skills[skill.Name()] = skill
		}
	}
}

// WithMaxRetries sets the maximum number of retries for tool execution.
func WithMaxRetries(max int) opt {
	return func(a *Agent) {
		a.maxRetries = max
	}
}

// NewAgent creates a new agent with the given configuration.
func NewAgent(session *session.Session, opts ...opt) (*Agent, error) {
	if session == nil {
		return nil, fmt.Errorf("session is required")
	}

	agent := &Agent{
		session:    session,
		tools:      make(map[string]Tool),
		skills:     make(map[string]Skill),
		maxRetries: 3,
	}

	for _, o := range opts {
		o(agent)
	}

	return agent, nil
}

// Execute processes a user request through the agent loop:
func (a *Agent) Execute(ctx context.Context, userInput string) (string, error) {
	// Build the enhanced prompt with memory context and tool information
	enhancedPrompt := a.buildPrompt(userInput)

	// Decision-making: initial LLM call
	response, err := a.session.Complete(ctx, llm.Message{
		Role:    llm.RoleUser,
		Content: enhancedPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("initial completion failed: %w", err)
	}

	// 4. Action & 5. Feedback: Execute tool calls and handle feedback
	return a.processResponse(ctx, response.Content, 0)
}

// processResponse handles tool calling and feedback loops.
func (a *Agent) processResponse(ctx context.Context, response string, retryCount int) (string, error) {
	// Parse the response for tool calls
	toolCalls := parseToolCalls(response)

	// If no tool calls, return the response as final answer
	if len(toolCalls) == 0 {
		return response, nil
	}

	// Execute all tool calls
	results := make([]toolResult, 0, len(toolCalls))
	hasErrors := false
	for _, call := range toolCalls {
		result := a.executeTool(ctx, call)
		results = append(results, result)
		if result.Error != "" {
			hasErrors = true
		}
	}

	// Build feedback prompt with tool results
	feedbackPrompt := a.buildFeedbackPrompt(toolCalls, results, hasErrors)

	// If we have errors and haven't exceeded retry limit, allow replanning
	if hasErrors && retryCount < a.maxRetries {
		feedbackPrompt += fmt.Sprintf("\n\nSome tools failed. Please replan or provide an alternative solution. (Attempt %d/%d)", retryCount+1, a.maxRetries)
	}

	// Get the next response from the LLM
	nextResponse, err := a.session.Complete(ctx, llm.Message{
		Role:    llm.RoleUser,
		Content: feedbackPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("feedback completion failed: %w", err)
	}

	// Recursive call to handle potential additional tool calls
	return a.processResponse(ctx, nextResponse.Content, retryCount+1)
}

// toolCall represents a request to invoke a specific tool.
type toolCall struct {
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}

// toolResult captures the outcome of a tool execution.
type toolResult struct {
	Tool   string          `json:"tool"`
	Output json.RawMessage `json:"output"`
	Error  string          `json:"error,omitempty"`
}

// executeTool executes a single tool call and returns the result.
// It automatically routes to tools or skills.
func (a *Agent) executeTool(ctx context.Context, call toolCall) toolResult {
	tool, ok := a.tools[call.Tool]
	if !ok {
		return toolResult{
			Tool:  call.Tool,
			Error: fmt.Sprintf("tool %q not found", call.Tool),
		}
	}

	output, err := tool.Execute(ctx, call.Input)
	if err != nil {
		return toolResult{
			Tool:  call.Tool,
			Error: err.Error(),
		}
	}
	return toolResult{
		Tool:   call.Tool,
		Output: output,
	}
}

// parseToolCalls extracts tool calls from the LLM response.
// Expected format: <tool_call>{"tool": "...", "input": {...}}</tool_call>
func parseToolCalls(response string) []toolCall {
	var calls []toolCall

	// Simple XML-like tag parsing
	for {
		start := strings.Index(response, "<tool_call>")
		if start == -1 {
			break
		}
		end := strings.Index(response[start:], "</tool_call>")
		if end == -1 {
			break
		}
		end += start

		callJSON := response[start+len("<tool_call>") : end]
		var call toolCall

		if n, err := jsonrepair.RepairJSON(callJSON); err == nil {
			callJSON = n
		}

		if err := json.NewDecoder(bytes.NewBuffer([]byte(callJSON))).Decode(&call); err == nil {
			if call.Tool != "" {
				calls = append(calls, call)
			} else {
				slog.Warn("tool call missing 'tool' field", "json", callJSON)
			}
		} else {
			slog.Warn("failed to parse tool call JSON", "error", err, "json", callJSON)
		}

		response = response[end+len("</tool_call>"):]
	}

	return calls
}

// buildPrompt constructs the initial prompt with memory and tool information.
func (a *Agent) buildPrompt(userInput string) string {
	var sb strings.Builder

	sb.WriteString("You are an intelligent agent that can use tools and skills to accomplish tasks.\n\n")

	// List available skills (higher-level capabilities)
	if len(a.skills) > 0 {
		sb.WriteString("## Available Skills (High-level capabilities):\n")
		for _, skill := range a.skills {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Path(), skill.Description()))
		}
	}

	// List available tools (low-level operations)
	if len(a.tools) > 0 {
		sb.WriteString("## Available Tools (Low-level operations):\n")
		for _, tool := range a.tools {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name(), tool.Description()))
		}
		sb.WriteString("\n")

		sb.WriteString("To use a tool, respond with: <tool_call>{\"tool\": \"name\", \"input\": {...}}</tool_call>\n")
		sb.WriteString("You can make multiple tool calls in a single response.\n")
		sb.WriteString("Prefer using skills for complex, multi-step operations when available.\n\n")
	}

	sb.WriteString("## User Request:\n")
	sb.WriteString(userInput)

	return sb.String()
}

// buildFeedbackPrompt constructs a feedback prompt with tool results.
func (a *Agent) buildFeedbackPrompt(calls []toolCall, results []toolResult, hasErrors bool) string {
	var sb strings.Builder

	sb.WriteString("## Tool Execution Results:\n\n")
	for i, result := range results {
		sb.WriteString(fmt.Sprintf("### Tool: %s\n", result.Tool))
		sb.WriteString(fmt.Sprintf("Input: %s\n", string(calls[i].Input)))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		} else {
			sb.WriteString(fmt.Sprintf("Output: %s\n", string(result.Output)))
		}
		sb.WriteString("\n")
	}

	if !hasErrors {
		sb.WriteString("Based on these results, provide a final response to the user.\n")
	}

	return sb.String()
}
