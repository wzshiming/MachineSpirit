package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Tool represents an action the agent can execute.
type Tool struct {
	Name        string
	Description string
	Fn          func(context.Context, string) (string, error)
}

// AgentConfig controls how the agent plans and executes tool calls.
type AgentConfig struct {
	Tools    []Tool
	MaxSteps int
}

// Agent drives multi-step reasoning over a Session, optionally invoking tools.
type Agent struct {
	session      *Session
	tools        map[string]Tool
	maxSteps     int
	systemPrompt string
}

type agentCommand struct {
	Action string `json:"action"`
	Tool   string `json:"tool,omitempty"`
	Input  string `json:"input,omitempty"`
	Reply  string `json:"reply,omitempty"`
}

const agentInstruction = `Follow a perception -> memory retrieval -> decision-making -> action -> feedback loop. If a tool is needed, reply with JSON {"action":"call_tool","tool":"<name>","input":"<input>"}. When ready to answer the user, reply with JSON {"action":"respond","reply":"<message>"}.`

// NewAgent constructs an Agent bound to an existing Session.
func NewAgent(session *Session, cfg AgentConfig) *Agent {
	toolMap := make(map[string]Tool)
	for _, tool := range cfg.Tools {
		if tool.Name == "" || tool.Fn == nil {
			continue
		}
		toolMap[tool.Name] = tool
	}

	maxSteps := cfg.MaxSteps
	if maxSteps == 0 {
		maxSteps = 8
	}

	systemPrompt := session.systemPrompt
	var toolDescriptions []string
	if len(toolMap) > 0 {
		names := make([]string, 0, len(toolMap))
		for name := range toolMap {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			tool := toolMap[name]
			toolDescriptions = append(toolDescriptions, fmt.Sprintf("%s: %s", tool.Name, tool.Description))
		}
	}
	if len(toolDescriptions) > 0 {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\nTools you can call:\n- " + strings.Join(toolDescriptions, "\n- ") + "\n\n" + agentInstruction)
	} else {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + agentInstruction + " No tools are available; respond directly.")
	}

	return &Agent{
		session:      session,
		tools:        toolMap,
		maxSteps:     maxSteps,
		systemPrompt: systemPrompt,
	}
}

// Run processes a user input, optionally invoking tools until a final reply is produced.
func (a *Agent) Run(ctx context.Context, input string) (Message, error) {
	if a == nil || a.session == nil {
		return Message{}, errors.New("agent session is required")
	}

	prompt := Message{Role: RoleUser, Content: strings.TrimSpace(input)}
	for step := 0; step < a.maxSteps; step++ {
		resp, err := a.session.complete(ctx, prompt, a.systemPrompt)
		if err != nil {
			return Message{}, err
		}

		cmd, ok := parseAgentCommand(resp.Content)
		if !ok {
			return resp, nil
		}

		switch strings.ToLower(cmd.Action) {
		case "respond", "reply", "final":
			if cmd.Reply != "" {
				resp.Content = cmd.Reply
			}
			return resp, nil
		case "call_tool", "tool":
			tool, ok := a.tools[cmd.Tool]
			if !ok {
				return Message{}, fmt.Errorf("unknown tool %q", cmd.Tool)
			}
			out, err := tool.Fn(ctx, cmd.Input)
			if err != nil {
				return Message{}, fmt.Errorf("tool %s failed: %w", tool.Name, err)
			}
			prompt = Message{
				Role:    RoleUser,
				Content: fmt.Sprintf("Tool %s result: %s", tool.Name, strings.TrimSpace(out)),
			}
		default:
			return resp, nil
		}
	}

	return Message{}, fmt.Errorf("agent exceeded max steps (%d)", a.maxSteps)
}

func parseAgentCommand(content string) (agentCommand, bool) {
	var cmd agentCommand
	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cmd); err != nil || cmd.Action == "" {
		return agentCommand{}, false
	}
	return cmd, true
}
