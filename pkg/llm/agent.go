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
	Short       string // short one-line summary
	Description string // longer detail
	Parameters  map[string]string
	Returns     map[string]string
	Fn          func(context.Context, map[string]string) (map[string]string, error)
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
	Input  any    `json:"input,omitempty"`
	Reply  string `json:"reply,omitempty"`
}

const agentInstruction = `Follow a perception -> memory retrieval -> decision-making -> action -> feedback loop. Always respond with raw JSON only (no XML/HTML/Markdown). If a tool is needed, reply with {"action":"call_tool","tool":"<name>","input":{<parameters as JSON object>}}. When ready to answer the user, reply with {"action":"respond","reply":"<message>"}.`

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func formatKV(kv map[string]string) string {
	if len(kv) == 0 {
		return ""
	}
	var keys []string
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, kv[k]))
	}
	return strings.Join(parts, "; ")
}

func normalizeInput(v any) (map[string]string, error) {
	if v == nil {
		return map[string]string{}, nil
	}
	if m, ok := v.(map[string]string); ok {
		return m, nil
	}
	// handle map[string]any
	if m, ok := v.(map[string]any); ok {
		out := make(map[string]string, len(m))
		for k, val := range m {
			out[k] = fmt.Sprint(val)
		}
		return out, nil
	}
	// handle raw JSON string/object encoded as string
	if s, ok := v.(string); ok {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			out := make(map[string]string, len(parsed))
			for k, val := range parsed {
				out[k] = fmt.Sprint(val)
			}
			return out, nil
		}
		return map[string]string{"input": s}, nil
	}

	return nil, fmt.Errorf("unsupported input type %T", v)
}

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
			short := firstNonEmpty(tool.Short, "no short description")
			long := firstNonEmpty(tool.Description, "no detailed description")
			params := firstNonEmpty(formatKV(tool.Parameters), "free-form text input")
			rets := firstNonEmpty(formatKV(tool.Returns), "unspecified")
			toolDescriptions = append(toolDescriptions,
				fmt.Sprintf("%s — %s\n  Details: %s\n  Parameters: %s\n  Returns: %s",
					tool.Name, short, long, params, rets))
		}
	}
	if len(toolDescriptions) > 0 {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\nTools you can call (short | details | parameters):\n- " + strings.Join(toolDescriptions, "\n- ") + "\n\nYou can ask for details again if unclear.\n\n" + agentInstruction)
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
			args, err := normalizeInput(cmd.Input)
			if err != nil {
				return Message{}, fmt.Errorf("invalid tool input: %w", err)
			}
			out, err := tool.Fn(ctx, args)
			if err != nil {
				return Message{}, fmt.Errorf("tool %s failed: %w", tool.Name, err)
			}
			if out == nil {
				out = map[string]string{}
			}
			outJSON, err := json.Marshal(out)
			if err != nil {
				return Message{}, fmt.Errorf("tool %s produced non-serializable output: %w", tool.Name, err)
			}
			prompt = Message{
				Role:    RoleUser,
				Content: fmt.Sprintf("Tool %s result: %s", tool.Name, strings.TrimSpace(string(outJSON))),
			}
		default:
			return resp, nil
		}
	}

	return Message{}, fmt.Errorf("agent exceeded max steps (%d)", a.maxSteps)
}

func parseAgentCommand(content string) (agentCommand, bool) {
	tryDecode := func(text string) (agentCommand, bool) {
		var cmd agentCommand
		dec := json.NewDecoder(strings.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cmd); err != nil || cmd.Action == "" {
			return agentCommand{}, false
		}
		return cmd, true
	}

	trimmed := strings.TrimSpace(content)
	if cmd, ok := tryDecode(trimmed); ok {
		return cmd, true
	}

	if strings.Contains(trimmed, "```") {
		first := strings.Index(trimmed, "```")
		if first >= 0 {
			rest := trimmed[first+3:]
			second := strings.Index(rest, "```")
			if second > 0 {
				if cmd, ok := tryDecode(strings.TrimSpace(rest[:second])); ok {
					return cmd, true
				}
			}
		}
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		if cmd, ok := tryDecode(strings.TrimSpace(trimmed[start : end+1])); ok {
			return cmd, true
		}
	}

	return agentCommand{}, false
}
