package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

// Agent orchestrates multi-step reasoning with tool calling and memory.
type Agent struct {
	session    *llm.Session
	memory     Memory
	tools      map[string]Tool
	invoker    *MultiToolInvoker
	maxRetries int
}

// Config configures an Agent.
type Config struct {
	// Session is the underlying LLM session.
	Session *llm.Session
	// Memory stores and retrieves facts. If nil, an in-memory store is created.
	Memory Memory
	// Tools are the actions available to the agent.
	Tools []Tool
	// Skills are high-level capabilities. If nil, an empty registry is created.
	Skills *SkillRegistry
	// MaxRetries is the maximum number of retry attempts on tool failure (default: 3).
	MaxRetries int
}

// NewAgent creates a new agent with the given configuration.
func NewAgent(cfg Config) (*Agent, error) {
	if cfg.Session == nil {
		return nil, fmt.Errorf("session is required")
	}

	memory := cfg.Memory
	if memory == nil {
		memory = NewInMemoryStore()
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	tools := make(map[string]Tool)
	for _, tool := range cfg.Tools {
		tools[tool.Name()] = tool
	}

	skills := cfg.Skills
	if skills == nil {
		skills = NewSkillRegistry()
	}

	// Create multi-tool invoker for routing between tools and skills
	invoker := NewMultiToolInvoker(tools, skills)

	return &Agent{
		session:    cfg.Session,
		memory:     memory,
		tools:      tools,
		invoker:    invoker,
		maxRetries: maxRetries,
	}, nil
}

// Execute processes a user request through the agent loop:
// 1. Perception: Receive user input
// 2. Memory retrieval: Search for relevant facts
// 3. Decision-making: LLM reasons and decides on actions
// 4. Action: Execute tool calls
// 5. Feedback: Process results and replan if needed
func (a *Agent) Execute(ctx context.Context, userInput string) (string, error) {
	// 1. Perception: receive user input
	// 2. Memory retrieval: search for relevant context
	relevantFacts := a.memory.Search(userInput)
	memoryContext := a.formatMemoryContext(relevantFacts)

	// Build the enhanced prompt with memory context and tool information
	enhancedPrompt := a.buildPrompt(userInput, memoryContext)

	// 3. Decision-making: initial LLM call
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
	toolCalls := a.parseToolCalls(response)

	// If no tool calls, return the response as final answer
	if len(toolCalls) == 0 {
		return response, nil
	}

	// Execute all tool calls
	results := make([]ToolResult, 0, len(toolCalls))
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

// executeTool executes a single tool call and returns the result.
// It uses the MultiToolInvoker to automatically route to tools or skills.
func (a *Agent) executeTool(ctx context.Context, call ToolCall) ToolResult {
	// Use invoker to automatically detect and route to tool or skill
	_, output, err := a.invoker.InvokeAuto(ctx, call.ToolName, call.Input)
	if err != nil {
		return ToolResult{
			ToolName: call.ToolName,
			Error:    err.Error(),
		}
	}

	return ToolResult{
		ToolName: call.ToolName,
		Output:   output,
	}
}

// parseToolCalls extracts tool calls from the LLM response.
// Expected format: <tool_call>{"tool_name": "...", "input": {...}}</tool_call>
func (a *Agent) parseToolCalls(response string) []ToolCall {
	var calls []ToolCall

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
		var call ToolCall
		if err := json.Unmarshal([]byte(callJSON), &call); err == nil {
			calls = append(calls, call)
		}

		response = response[end+len("</tool_call>"):]
	}

	return calls
}

// buildPrompt constructs the initial prompt with memory and tool information.
func (a *Agent) buildPrompt(userInput, memoryContext string) string {
	var sb strings.Builder

	sb.WriteString("You are an intelligent agent that can use tools and skills to accomplish tasks.\n\n")

	if memoryContext != "" {
		sb.WriteString("## Relevant Context from Memory:\n")
		sb.WriteString(memoryContext)
		sb.WriteString("\n\n")
	}

	// List available skills (higher-level capabilities)
	skills := a.invoker.GetSkillInvoker().List()
	if len(skills) > 0 {
		// Separate instruction-based skills from executable skills
		var instructionSkills []Skill
		var executableSkills []Skill

		for _, skillName := range skills {
			if skill, err := a.invoker.GetSkillInvoker().registry.Get(skillName); err == nil {
				// Check if this is an instruction-based skill
				if instrSkill, ok := skill.(interface{ IsInstructionBased() bool }); ok && instrSkill.IsInstructionBased() {
					instructionSkills = append(instructionSkills, skill)
				} else {
					executableSkills = append(executableSkills, skill)
				}
			}
		}

		// Include instruction-based skills with full instructions
		if len(instructionSkills) > 0 {
			sb.WriteString("## Expert Skills (Follow these instructional guides):\n\n")
			for _, skill := range instructionSkills {
				sb.WriteString(skill.DetailedDescription())
				sb.WriteString("\n\n---\n\n")
			}
		}

		// List executable skills briefly
		if len(executableSkills) > 0 {
			sb.WriteString("## Available Workflows (Executable capabilities):\n")
			for _, skill := range executableSkills {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Name(), skill.Description()))
			}
			sb.WriteString("\n")
		}
	}

	// List available tools (low-level operations)
	if len(a.tools) > 0 {
		sb.WriteString("## Available Tools (Low-level operations):\n")
		for _, tool := range a.tools {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name(), tool.Description()))
		}
		sb.WriteString("\n")
	}

	if len(skills) > 0 || len(a.tools) > 0 {
		sb.WriteString("To use a tool or skill, respond with: <tool_call>{\"tool_name\": \"name\", \"input\": {...}}</tool_call>\n")
		sb.WriteString("You can make multiple tool calls in a single response.\n")
		sb.WriteString("Prefer using skills for complex, multi-step operations when available.\n\n")
	}

	sb.WriteString("## User Request:\n")
	sb.WriteString(userInput)

	return sb.String()
}

// buildFeedbackPrompt constructs a feedback prompt with tool results.
func (a *Agent) buildFeedbackPrompt(calls []ToolCall, results []ToolResult, hasErrors bool) string {
	var sb strings.Builder

	sb.WriteString("## Tool Execution Results:\n\n")
	for i, result := range results {
		sb.WriteString(fmt.Sprintf("### Tool: %s\n", result.ToolName))
		sb.WriteString(fmt.Sprintf("Input: %s\n", string(calls[i].Input)))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		} else {
			sb.WriteString(fmt.Sprintf("Output: %s\n", result.Output))
		}
		sb.WriteString("\n")
	}

	if !hasErrors {
		sb.WriteString("Based on these results, provide a final response to the user.\n")
	}

	return sb.String()
}

// formatMemoryContext formats facts into a readable context string.
func (a *Agent) formatMemoryContext(facts []Fact) string {
	if len(facts) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, fact := range facts {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", fact.Key, fact.Value))
	}
	return sb.String()
}

// Memory returns the agent's memory store.
func (a *Agent) Memory() Memory {
	return a.memory
}

// RegisterTool adds a tool to the agent's available tools.
func (a *Agent) RegisterTool(tool Tool) {
	a.tools[tool.Name()] = tool
	// Update invoker with new tool
	a.invoker = NewMultiToolInvoker(a.tools, a.invoker.GetSkillInvoker().registry)
}

// RegisterSkill adds a skill to the agent's available skills.
func (a *Agent) RegisterSkill(skill Skill) error {
	if err := a.invoker.GetSkillInvoker().registry.Register(skill); err != nil {
		return err
	}
	return nil
}

// GetSkillRegistry returns the agent's skill registry.
func (a *Agent) GetSkillRegistry() *SkillRegistry {
	return a.invoker.GetSkillInvoker().registry
}

// GetInvoker returns the agent's multi-tool invoker.
func (a *Agent) GetInvoker() *MultiToolInvoker {
	return a.invoker
}

// LoadSkillsFromDirectory loads markdown-based skills from a directory.
func (a *Agent) LoadSkillsFromDirectory(skillsDir string) error {
	loader := NewSkillLoader(skillsDir)
	skills, err := loader.LoadAllSkills()
	if err != nil {
		return fmt.Errorf("failed to load skills: %w", err)
	}

	for _, skill := range skills {
		if err := a.RegisterSkill(skill); err != nil {
			return fmt.Errorf("failed to register skill %s: %w", skill.Name(), err)
		}
	}

	return nil
}
