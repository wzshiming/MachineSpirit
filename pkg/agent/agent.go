package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	jsonrepair "github.com/RealAlexandreAI/json-repair"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/persistence/i18n"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

// defaultInputQueueSize is the default buffer size for the input queue.
const defaultInputQueueSize = 64

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
	pm          *persistence.PersistenceManager
	session     *session.Session
	tools       map[string]Tool
	skills      *skills.Skills
	maxRetries  int
	strings     agentStrings
	mut         sync.Mutex
	inputQueue  chan llm.Message
	inputNotify chan struct{}
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
		session:     session,
		tools:       make(map[string]Tool),
		maxRetries:  3,
		inputQueue:  make(chan llm.Message, defaultInputQueueSize),
		inputNotify: make(chan struct{}, 1),
	}

	for _, o := range opts {
		o(agent)
	}

	agent.strings = englishStrings()

	return agent, nil
}

// Execute processes a user request through the agent loop:
func (a *Agent) Execute(ctx context.Context, userInput string, output io.Writer) error {
	a.mut.Lock()
	defer a.mut.Unlock()

	// Build the enhanced prompt with memory context and tool information
	enhancedPrompt := a.buildPrompt(userInput)

	// Decision-making: initial LLM call
	response, err := a.session.Complete(ctx,
		session.SessionRequest{
			SystemPrompt: a.BuildSystemPrompt(),
			Prompt: session.Message{
				Role:    session.RoleUser,
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

	// Get the next response from the LLM
	nextResponse, err := a.session.Complete(ctx,
		session.SessionRequest{
			SystemPrompt: a.BuildSystemPrompt(),
			Prompt: session.Message{
				Role:    session.RoleUser,
				Content: feedbackPrompt,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("feedback completion failed: %w", err)
	}

	return a.processResponse(ctx, output, nextResponse.Content, retryCount+1)
}

// AddInput enqueues a message into the agent's input queue.
// This allows external sources (such as sub-sessions) to inject messages
// while the agent is actively processing another request.
// It is safe to call from any goroutine.
func (a *Agent) AddInput(msg llm.Message) {
	select {
	case a.inputQueue <- msg:
	default:
		slog.Warn("Agent input queue is full, dropping message", "role", msg.Role)
		return
	}
	// Signal that a new message is available.
	select {
	case a.inputNotify <- struct{}{}:
	default:
		// Already signalled; the consumer will drain all pending messages.
	}
}

// DrainInputs returns all currently pending messages from the input queue
// without blocking. Returns nil if no messages are pending.
func (a *Agent) DrainInputs() []llm.Message {
	var msgs []llm.Message
	for {
		select {
		case msg := <-a.inputQueue:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// HasPendingInputs reports whether there are messages waiting in the input queue.
func (a *Agent) HasPendingInputs() bool {
	return len(a.inputQueue) > 0
}

// InputNotify returns a channel that receives a value each time AddInput
// enqueues a new message. Consumers should drain all pending messages via
// DrainInputs after each receive, as the channel is buffered with size 1
// and multiple AddInput calls may coalesce into a single notification.
func (a *Agent) InputNotify() <-chan struct{} {
	return a.inputNotify
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

// BuildSystemPrompt constructs the full system prompt including workspace
// context from persistence files and the agent's available skills and tools.
func (a *Agent) BuildSystemPrompt() string {
	var parts []string

	if a.pm != nil {
		baseDir := a.pm.GetBaseDir()

		items := []string{
			i18n.FileAgents,
		}

		bootstrap, _ := a.pm.CheckBootstrap()
		if bootstrap {
			items = append(items, i18n.FileBootstrap)
			parts = append(parts, "NOTE: Execute the `BOOTSTRAP.md` process as defined in the workspace.")
		}

		now := time.Now()
		zone, offset := now.Zone()
		parts = append(parts, fmt.Sprintf("Current time %s, zone %s (UTC%+d)", now.Format(time.RFC3339), zone, offset/3600))

		parts = append(parts, fmt.Sprintf("Workspace %s", baseDir))

		// files of baseDir
		entrys, err := os.ReadDir(baseDir)
		if err != nil {
			slog.Warn("Failed to read workspace directory", "dir", baseDir, "error", err)
		}

		list := make([]string, 0, len(entrys))
		for _, entry := range entrys {
			if entry.IsDir() {
				list = append(list, entry.Name()+"/")
			} else {
				list = append(list, entry.Name())
			}
		}

		parts = append(parts, "Workspace files:\n"+strings.Join(list, "\n"))

		for _, item := range items {
			content, err := a.pm.ReadFile(item)
			if err != nil {
				if !os.IsNotExist(err) {
					slog.Warn("Failed to read persistence file", "file", item, "error", err)
				}
				continue
			}
			if content != "" {
				parts = append(parts, content)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(parts, "\n\n"))
	sb.WriteString("\n\n")

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
		hasCompress := false
		for _, tool := range a.tools {
			if !tool.Enabled() {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name(), tool.Description()))
			sb.WriteString(FormatToolParameters(tool.Parameters()))
			if !hasSubSession && tool.Name() == "sub_session" {
				hasSubSession = true
			}
			if !hasCompress && tool.Name() == "compress_transcript" {
				hasCompress = true
			}
		}
		sb.WriteString(a.strings.ToolCallInstructions)
		sb.WriteString(a.strings.MultipleToolCallsHint)
		if hasSubSession {
			sb.WriteString(a.strings.SubSessionHint)
		}
		if hasCompress {
			sb.WriteString(a.strings.CompressHint)
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
