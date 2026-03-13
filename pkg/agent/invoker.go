package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// Invoker defines the interface for executing tools or skills.
type Invoker interface {
	// Invoke executes the named action with the given input.
	Invoke(ctx context.Context, name string, input json.RawMessage) (string, error)
	// Has checks if the invoker can handle the named action.
	Has(name string) bool
	// List returns all available actions.
	List() []string
}

// ToolInvoker invokes low-level tools.
type ToolInvoker struct {
	tools map[string]Tool
}

// NewToolInvoker creates a new tool invoker.
func NewToolInvoker(tools map[string]Tool) *ToolInvoker {
	if tools == nil {
		tools = make(map[string]Tool)
	}
	return &ToolInvoker{tools: tools}
}

func (i *ToolInvoker) Invoke(ctx context.Context, name string, input json.RawMessage) (string, error) {
	tool, exists := i.tools[name]
	if !exists {
		return "", fmt.Errorf("tool %q not found", name)
	}
	return tool.Execute(ctx, input)
}

func (i *ToolInvoker) Has(name string) bool {
	_, exists := i.tools[name]
	return exists
}

func (i *ToolInvoker) List() []string {
	names := make([]string, 0, len(i.tools))
	for name := range i.tools {
		names = append(names, name)
	}
	return names
}

// SkillInvoker invokes high-level skills.
type SkillInvoker struct {
	registry *SkillRegistry
}

// NewSkillInvoker creates a new skill invoker.
func NewSkillInvoker(registry *SkillRegistry) *SkillInvoker {
	if registry == nil {
		registry = NewSkillRegistry()
	}
	return &SkillInvoker{registry: registry}
}

func (i *SkillInvoker) Invoke(ctx context.Context, name string, input json.RawMessage) (string, error) {
	skill, err := i.registry.Get(name)
	if err != nil {
		return "", err
	}
	return skill.Execute(ctx, input)
}

func (i *SkillInvoker) Has(name string) bool {
	return i.registry.Has(name)
}

func (i *SkillInvoker) List() []string {
	skills := i.registry.List()
	names := make([]string, len(skills))
	for i, skill := range skills {
		names[i] = skill.Name()
	}
	return names
}

// MultiToolInvoker routes tool/skill calls to the appropriate invoker.
type MultiToolInvoker struct {
	toolInvoker  *ToolInvoker
	skillInvoker *SkillInvoker
}

// NewMultiToolInvoker creates a new multi-tool invoker.
func NewMultiToolInvoker(tools map[string]Tool, skills *SkillRegistry) *MultiToolInvoker {
	return &MultiToolInvoker{
		toolInvoker:  NewToolInvoker(tools),
		skillInvoker: NewSkillInvoker(skills),
	}
}

// Invoke executes a tool or skill by name.
// It first checks skills, then falls back to tools.
func (m *MultiToolInvoker) Invoke(ctx context.Context, kind ToolKind, name string, input json.RawMessage) (string, error) {
	switch kind {
	case ToolKindSkill:
		return m.skillInvoker.Invoke(ctx, name, input)
	case ToolKindTool, ToolKindBuiltIn:
		return m.toolInvoker.Invoke(ctx, name, input)
	default:
		// Auto-detect: try skills first, then tools
		if m.skillInvoker.Has(name) {
			return m.skillInvoker.Invoke(ctx, name, input)
		}
		if m.toolInvoker.Has(name) {
			return m.toolInvoker.Invoke(ctx, name, input)
		}
		return "", fmt.Errorf("action %q not found in any invoker", name)
	}
}

// InvokeAuto automatically detects whether to use a skill or tool.
func (m *MultiToolInvoker) InvokeAuto(ctx context.Context, name string, input json.RawMessage) (ToolKind, string, error) {
	// Try skills first (higher-level)
	if m.skillInvoker.Has(name) {
		output, err := m.skillInvoker.Invoke(ctx, name, input)
		return ToolKindSkill, output, err
	}
	// Fall back to tools
	if m.toolInvoker.Has(name) {
		output, err := m.toolInvoker.Invoke(ctx, name, input)
		return ToolKindTool, output, err
	}
	return "", "", fmt.Errorf("action %q not found", name)
}

// Has checks if any invoker can handle the named action.
func (m *MultiToolInvoker) Has(name string) bool {
	return m.skillInvoker.Has(name) || m.toolInvoker.Has(name)
}

// ListAll returns all available actions from all invokers.
func (m *MultiToolInvoker) ListAll() map[ToolKind][]string {
	return map[ToolKind][]string{
		ToolKindSkill: m.skillInvoker.List(),
		ToolKindTool:  m.toolInvoker.List(),
	}
}

// GetToolInvoker returns the tool invoker.
func (m *MultiToolInvoker) GetToolInvoker() *ToolInvoker {
	return m.toolInvoker
}

// GetSkillInvoker returns the skill invoker.
func (m *MultiToolInvoker) GetSkillInvoker() *SkillInvoker {
	return m.skillInvoker
}
