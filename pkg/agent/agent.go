package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	jsonrepair "github.com/RealAlexandreAI/json-repair"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// defaultCompressThreshold is the default number of transcript messages
// before the agent automatically triggers compression.
const defaultCompressThreshold = 50

// defaultAutoCompressKeepRecent is the number of recent messages to keep
// when auto-compression triggers.
const defaultAutoCompressKeepRecent = 10

// DefaultCompressSystemPrompt is the default prompt used to instruct the LLM
// how to summarize older conversation messages during transcript compression.
const DefaultCompressSystemPrompt = `You are summarizing a conversation transcript. Create a concise summary that preserves:
- Key decisions and their reasoning
- Important facts, state, and context established
- Task progress and outcomes
- Any pending or incomplete items
Write the summary as a brief narrative that can serve as context for continuing the conversation.`

// Agent orchestrates multi-step reasoning with tool calling and memory.
type Agent struct {
	pm                *persistence.PersistenceManager
	session           *session.Session
	tools             map[string]Tool
	skills            *skills.Skills
	maxRetries        int
	compressThreshold int
	strings           AgentStrings
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

// WithCompressThreshold sets the transcript message count threshold for
// automatic compression. When the session transcript exceeds this count,
// the agent will compress it before the next LLM call.
// A value of 0 or negative disables auto-compression.
func WithCompressThreshold(threshold int) opt {
	return func(a *Agent) {
		a.compressThreshold = threshold
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
		session:           session,
		tools:             make(map[string]Tool),
		maxRetries:        3,
		compressThreshold: defaultCompressThreshold,
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
func (a *Agent) Execute(ctx context.Context, userInput string, output io.Writer) error {
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
		return fmt.Errorf("initial completion failed: %w", err)
	}

	// 4. Action & 5. Feedback: Execute tool calls and handle feedback
	return a.processResponse(ctx, output, response.Content, 0)
}

// processResponse handles tool calling and feedback loops.
func (a *Agent) processResponse(ctx context.Context, output io.Writer, response string, retryCount int) error {
	// Parse the response for tool calls
	parsed := parseToolCalls(response)

	if parsed.NonToolContent != "" {
		_, err := io.WriteString(output, parsed.NonToolContent+"\n")
		if err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	// If no tool calls, return the response as final answer
	if len(parsed.ToolCalls) == 0 {
		return nil
	}

	// Execute all tool calls
	results := make([]toolResult, 0, len(parsed.ToolCalls))
	hasErrors := false
	for _, call := range parsed.ToolCalls {
		result := a.executeTool(ctx, call)
		results = append(results, result)
		if result.Error != "" {
			hasErrors = true
		}
	}

	// Build feedback prompt with tool results
	feedbackPrompt := a.buildFeedbackPrompt(parsed.ToolCalls, results, hasErrors)

	// If we have errors and haven't exceeded retry limit, allow replanning
	if hasErrors && retryCount < a.maxRetries {
		feedbackPrompt += fmt.Sprintf(a.strings.ReplanPrompt, retryCount+1, a.maxRetries)
	}

	// Auto-compress if transcript has grown past the threshold
	a.maybeAutoCompress(ctx)

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
		return fmt.Errorf("feedback completion failed: %w", err)
	}

	return a.processResponse(ctx, output, nextResponse.Content, retryCount+1)
}

// maybeAutoCompress checks the session transcript size and automatically
// compresses it if it exceeds the configured threshold. This prevents the
// context window from growing unbounded during long agent interactions.
func (a *Agent) maybeAutoCompress(ctx context.Context) {
	if a.compressThreshold <= 0 {
		return
	}
	if a.session.Size() <= a.compressThreshold {
		return
	}

	slog.Info("Auto-compressing transcript",
		"size", a.session.Size(),
		"threshold", a.compressThreshold,
	)

	_, err := a.session.CompressTranscript(ctx, defaultAutoCompressKeepRecent, DefaultCompressSystemPrompt)
	if err != nil {
		slog.Warn("Auto-compression failed", "error", err)
	}
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

// parsedResponse holds the result of parsing an LLM response for tool calls.
type parsedResponse struct {
	NonToolContent string     // Text outside <tool_call>...</tool_call> tags
	ToolCalls      []toolCall // Extracted tool calls
}

// parseToolCalls extracts tool calls from the LLM response and collects
// the surrounding non-tool text so it can be relayed to the user.
// Expected format: <tool_call name="...">{...}</tool_call>
func parseToolCalls(response string) parsedResponse {
	var calls []toolCall
	var nonToolContent strings.Builder
	remaining := response

	// Simple XML-like tag parsing
	for {
		start := strings.Index(remaining, "<tool_call name=\"")
		if start == -1 {
			nonToolContent.WriteString(remaining)
			break
		}

		// Validate that <tool_call is followed by a space or '>',
		// not other characters (e.g. when <tool_call appears inside code/strings).
		nextCharIdx := start + len("<tool_call")
		if nextCharIdx >= len(remaining) {
			nonToolContent.WriteString(remaining)
			break
		}
		nextChar := remaining[nextCharIdx]
		if nextChar != ' ' && nextChar != '>' {
			nonToolContent.WriteString(remaining[:nextCharIdx])
			remaining = remaining[nextCharIdx:]
			continue
		}

		// Find end of opening tag
		tagEnd := strings.Index(remaining[start:], ">")
		if tagEnd == -1 {
			nonToolContent.WriteString(remaining)
			break
		}
		tagEnd += start

		// Reject tags that span multiple lines (likely false matches from code output).
		if strings.ContainsAny(remaining[start:tagEnd], "\n\r") {
			nonToolContent.WriteString(remaining[:start+len("<tool_call")])
			remaining = remaining[start+len("<tool_call"):]
			continue
		}

		// Find closing tag
		end := strings.Index(remaining[tagEnd:], "</tool_call>")
		if end == -1 {
			nonToolContent.WriteString(remaining)
			break
		}
		end += tagEnd

		// Collect non-tool text before this tool call
		nonToolContent.WriteString(remaining[:start])

		openTag := remaining[start : tagEnd+1]
		callJSON := strings.TrimSpace(remaining[tagEnd+1 : end])

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
				nonToolContent.WriteString(fmt.Sprintf("[Tool call: %s, input: %s]\n", toolName, string(input)))
			} else {
				slog.Warn("failed to parse tool call JSON", "error", err, "json", callJSON)
			}
		} else {
			slog.Warn("tool call missing 'name' attribute", "tag", openTag)
		}

		remaining = remaining[end+len("</tool_call>"):]
	}

	return parsedResponse{
		NonToolContent: strings.TrimSpace(nonToolContent.String()),
		ToolCalls:      calls,
	}
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
		hasSubSession := false
		for _, tool := range a.tools {
			if !tool.Enabled() {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name(), tool.Description()))
			sb.WriteString(FormatToolParameters(tool.Parameters()))
			if tool.Name() == "sub_session" {
				hasSubSession = true
			}
		}
		sb.WriteString(a.strings.ToolCallInstructions)
		sb.WriteString(a.strings.MultipleToolCallsHint)
		if hasSubSession {
			sb.WriteString(a.strings.SubSessionHint)
		}
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
