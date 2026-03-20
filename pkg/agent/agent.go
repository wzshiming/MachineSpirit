package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	jsonrepair "github.com/RealAlexandreAI/json-repair"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// Agent orchestrates multi-step reasoning with tool calling and memory.
type Agent struct {
	pm         *persistence.PersistenceManager
	session    *session.Session
	tools      map[string]Tool
	skills     *skills.Skills
	maxRetries int
	strings    AgentStrings
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
func WithSkills(skills *skills.Skills) opt {
	return func(a *Agent) {
		a.skills = skills
	}
}

// WithMaxRetries sets the maximum number of retries for tool execution.
func WithMaxRetries(max int) opt {
	return func(a *Agent) {
		a.maxRetries = max
	}
}

// WithPersistenceManager sets the persistence manager for the agent.
func WithPersistenceManager(pm *persistence.PersistenceManager) opt {
	return func(a *Agent) {
		a.pm = pm
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
		maxRetries: 3,
	}

	for _, o := range opts {
		o(agent)
	}

	// Initialize localized strings based on persistence manager's locale
	locale := "en"
	if agent.pm != nil {
		locale = agent.pm.GetLocale()
	}
	agent.strings = GetStrings(locale)

	return agent, nil
}

// Execute processes a user request through the agent loop:
func (a *Agent) Execute(ctx context.Context, userInput string) (string, error) {
	// Build the enhanced prompt with memory context and tool information
	enhancedPrompt := a.buildPrompt(userInput)

	// Decision-making: initial LLM call
	response, err := a.session.Complete(ctx,
		llm.ChatRequest{
			SystemPrompt: a.buildSystemPrompt(),
			Prompt: llm.Message{
				Role:    llm.RoleUser,
				Content: enhancedPrompt,
			},
		},
	)
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
		feedbackPrompt += fmt.Sprintf(a.strings.ReplanPrompt, retryCount+1, a.maxRetries)
	}

	// Get the next response from the LLM
	nextResponse, err := a.session.Complete(ctx,
		llm.ChatRequest{
			SystemPrompt: a.buildSystemPrompt(),
			Prompt: llm.Message{
				Role:    llm.RoleUser,
				Content: feedbackPrompt,
			},
		},
	)

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
// Expected format: <tool_call name="...">{...}</tool_call>
func parseToolCalls(response string) []toolCall {
	var calls []toolCall

	// Simple XML-like tag parsing
	for {
		start := strings.Index(response, "<tool_call")
		if start == -1 {
			break
		}

		// Validate that <tool_call is followed by a space or '>',
		// not other characters (e.g. when <tool_call appears inside code/strings).
		nextCharIdx := start + len("<tool_call")
		if nextCharIdx >= len(response) {
			break
		}
		nextChar := response[nextCharIdx]
		if nextChar != ' ' && nextChar != '>' {
			response = response[nextCharIdx:]
			continue
		}

		// Find end of opening tag
		tagEnd := strings.Index(response[start:], ">")
		if tagEnd == -1 {
			break
		}
		tagEnd += start

		// Reject tags that span multiple lines (likely false matches from code output).
		if strings.ContainsAny(response[start:tagEnd], "\n\r") {
			response = response[start+len("<tool_call"):]
			continue
		}

		// Find closing tag
		end := strings.Index(response[tagEnd:], "</tool_call>")
		if end == -1 {
			break
		}
		end += tagEnd

		openTag := response[start : tagEnd+1]
		callJSON := strings.TrimSpace(response[tagEnd+1 : end])

		// Extract tool name from the tag attribute
		toolName := extractTagAttribute(openTag, "name")

		if n, err := jsonrepair.RepairJSON(callJSON); err == nil {
			callJSON = n
		}

		if toolName != "" {
			// New format: tool name in tag attribute, JSON body is just the input
			var input json.RawMessage
			if err := json.Unmarshal([]byte(callJSON), &input); err == nil {
				calls = append(calls, toolCall{Tool: toolName, Input: input})
			} else {
				slog.Warn("failed to parse tool call JSON", "error", err, "json", callJSON)
			}
		} else {
			slog.Warn("tool call missing 'name' attribute", "tag", openTag)
		}

		response = response[end+len("</tool_call>"):]
	}

	return calls
}

// extractTagAttribute extracts the value of a named attribute from an XML-like tag string.
// For example, extractTagAttribute(`<tool_call name="bash">`, "name") returns "bash".
func extractTagAttribute(tag, attr string) string {
	search := attr + `="`
	idx := strings.Index(tag, search)
	if idx == -1 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(tag[start:], `"`)
	if end == -1 {
		return ""
	}
	return tag[start : start+end]
}

func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(a.pm.BuildSystemPrompt(""))

	// List available skills (higher-level capabilities)
	if list := a.skills.List(); len(list) != 0 {
		sb.WriteString(a.strings.AvailableSkillsHeader)
		for _, skill := range list {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Path(), skill.Description()))
		}
		sb.WriteString(a.strings.UseSkillsHint)
	}

	// List available tools (low-level operations)
	if len(a.tools) > 0 {
		sb.WriteString(a.strings.AvailableToolsHeader)
		for _, tool := range a.tools {
			if !tool.Enabled() {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name(), tool.Description()))
		}
		sb.WriteString(a.strings.ToolCallInstructions)
		sb.WriteString(a.strings.MultipleToolCallsHint)
		sb.WriteString(a.strings.PreferSkillsHint)
	}

	return sb.String()
}

// buildPrompt constructs the initial prompt with memory and tool information.
func (a *Agent) buildPrompt(userInput string) string {
	var sb strings.Builder

	sb.WriteString(userInput)

	return sb.String()
}

// buildFeedbackPrompt constructs a feedback prompt with tool results.
func (a *Agent) buildFeedbackPrompt(calls []toolCall, results []toolResult, hasErrors bool) string {
	var sb strings.Builder

	for i, result := range results {
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("<tool_result name=%q>\n", result.Tool))
			sb.WriteString(fmt.Sprintf("### Input: %s\n", string(calls[i].Input)))
			sb.WriteString(fmt.Sprintf("### Error: %s\n", result.Error))
			sb.WriteString("</tool_result>\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("<tool_result name=%q>\n", result.Tool))
			sb.WriteString(fmt.Sprintf("%s\n", string(result.Output)))
			sb.WriteString("</tool_result>\n\n")
		}
	}

	if !hasErrors {
		sb.WriteString(a.strings.FinalResponsePrompt)
	}

	return sb.String()
}
