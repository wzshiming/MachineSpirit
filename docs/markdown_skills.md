# Markdown-Based Skills (Anthropic Pattern)

This document describes MachineSpirit's support for markdown-based instruction skills, following the Anthropic Skills framework pattern.

## Overview

In addition to executable code-based skills, MachineSpirit now supports **instruction-based skills** loaded from markdown files. This follows the Anthropic Skills pattern where skills are instructional guides that teach the LLM how to approach tasks, rather than executable code.

## Two Types of Skills

MachineSpirit supports both paradigms:

| Type | Format | Purpose | Example |
|------|---------|---------|---------|
| **Instruction Skills** | Markdown files | Teach LLM how to use tools | Flight booking guide with workflow |
| **Executable Skills** | Go code | Direct execution | FlightBookingSkill that runs code |

Both types work together seamlessly through the unified Skill interface.

## Markdown Skill Format

Markdown skills follow this structure:

```markdown
---
name: skill_name
description: Brief description of what this skill does
license: MIT
tags:
  - tag1
  - tag2
memory:
  key1: Description of what to remember
  key2: Another memory hint
---

# Skill Name

Detailed instructions in markdown format.

## When to Use This Skill

Guidelines on when to apply this skill...

## Available Tools

List of tools this skill can use...

## Workflow

Step-by-step process...
```

### Frontmatter Fields

- `name` (required): Unique identifier for the skill
- `description` (required): Brief description shown in listings
- `license` (optional): License for the skill
- `tags` (optional): Categorization tags
- `memory` (optional): Hints about what information to store/retrieve

## Creating Markdown Skills

### 1. Create Skill Directory

```bash
mkdir -p skills/my_skill
```

### 2. Create SKILL.md File

```bash
cat > skills/my_skill/SKILL.md << 'EOF'
---
name: my_skill
description: Expert guidance for my domain
license: MIT
memory:
  user_preference: Remember user's preferred approach
---

# My Skill Expert

This skill guides you through...

## When to Use

Use when the user asks for...

## Available Tools

- `tool_name` - Description

## Workflow

1. Step one
2. Step two
...

EOF
```

### 3. Load in Agent

```go
agent, _ := agent.NewAgent(agent.Config{
    Session: session,
    Tools:   tools,
})

// Load markdown skills from directory
agent.LoadSkillsFromDirectory("skills")
```

## How Instruction Skills Work

### 1. Loading

```go
loader := agent.NewSkillLoader("skills")
skills, err := loader.LoadAllSkills()
// Skills are parsed from SKILL.md files
```

### 2. Registration

```go
for _, skill := range skills {
    agent.RegisterSkill(skill)
}
```

### 3. Prompt Inclusion

When building prompts, the agent includes full instructions from markdown skills:

```
## Expert Skills (Follow these instructional guides):

# Flight Booking Expert

This skill guides you through helping users book flight tickets...

[Full markdown content included]

---
```

### 4. LLM Following Instructions

The LLM reads the instructions and follows the workflow described, using available tools as directed.

## Memory Integration

Markdown skills can specify memory hints in frontmatter:

```yaml
memory:
  preferred_airline: Store user's preferred airline
  frequent_routes: Remember common travel routes
```

The instructions then reference memory:

```markdown
### Before Searching

Check memory for `preferred_airline` and prioritize those flights.

### After Booking

Store the airline if user was satisfied: "I'll remember you prefer Delta."
```

## Example: Flight Booking Skill

See `skills/flight_booking/SKILL.md` for a complete example that demonstrates:

- **When to use**: Triggers for skill activation
- **Tool orchestration**: Using flight_search and flight_reservation
- **Memory integration**: Checking and storing preferences
- **Error handling**: Dealing with failures
- **Example interactions**: Step-by-step walkthroughs

## Differences from Executable Skills

| Aspect | Instruction Skills | Executable Skills |
|--------|-------------------|-------------------|
| **Definition** | Markdown files | Go code |
| **Loading** | Runtime from filesystem | Compiled into binary |
| **Execution** | LLM follows instructions | Code runs directly |
| **Modification** | Edit markdown, no recompile | Edit code, must recompile |
| **Flexibility** | Easy to customize per deployment | Type-safe, performant |
| **Use Case** | Guidance, workflows, best practices | Complex logic, API integration |

## Best Practices

### When to Use Instruction Skills

✅ **Good for:**
- Workflow guidance and best practices
- Domain-specific knowledge and procedures
- Multi-tool orchestration patterns
- Customizable per-deployment behaviors
- Teaching LLM new approaches

❌ **Not ideal for:**
- Complex computational logic
- Performance-critical operations
- Type-safe guarantees needed
- Direct API integrations

### Writing Effective Instructions

1. **Be specific**: Clear step-by-step workflows
2. **Use examples**: Show concrete interactions
3. **Reference tools**: Name specific tools to use
4. **Include memory**: Guide what to remember and why
5. **Handle errors**: Describe failure scenarios and recovery

### Organizing Skills

```
skills/
├── flight_booking/
│   └── SKILL.md
├── hotel_reservation/
│   └── SKILL.md
└── travel_planning/
    └── SKILL.md
```

Each skill in its own directory with a `SKILL.md` file.

## API Reference

### MarkdownSkill

```go
type MarkdownSkill struct {
    name         string
    description  string
    instructions string
    metadata     map[string]string
}

func NewMarkdownSkill(name, description, instructions string, metadata map[string]string) *MarkdownSkill
func (s *MarkdownSkill) Name() string
func (s *MarkdownSkill) Description() string
func (s *MarkdownSkill) DetailedDescription() string  // Returns instructions
func (s *MarkdownSkill) Instructions() string
func (s *MarkdownSkill) IsInstructionBased() bool     // Returns true
```

### SkillLoader

```go
type SkillLoader struct {
    skillsDir string
}

func NewSkillLoader(skillsDir string) *SkillLoader
func (l *SkillLoader) LoadSkill(path string) (*MarkdownSkill, error)
func (l *SkillLoader) LoadAllSkills() ([]*MarkdownSkill, error)
```

### Agent Methods

```go
func (a *Agent) LoadSkillsFromDirectory(skillsDir string) error
func (a *Agent) RegisterSkill(skill Skill) error
```

## Testing

```go
func TestMarkdownSkillLoading(t *testing.T) {
    loader := agent.NewSkillLoader("testdata/skills")
    skills, err := loader.LoadAllSkills()
    // Test skill loading and parsing
}
```

See `pkg/agent/markdown_skill_test.go` for comprehensive examples.

## Migration from Anthropic Skills

If you have existing Anthropic Skills files:

1. **Copy skills directory** to your project
2. **Verify format** matches the structure above
3. **Load in agent**: `agent.LoadSkillsFromDirectory("skills")`
4. **Test**: Skills are automatically registered and used

MachineSpirit's format is compatible with Anthropic's pattern with these notes:

- The markdown content is included verbatim in prompts
- Memory hints in frontmatter guide the agent's memory usage
- Skills work alongside executable Go skills seamlessly

## Combining Both Skill Types

You can use both instruction and executable skills together:

```go
// Register executable skills (code-based)
executableSkill := agent.NewFlightBookingSkill(searchTool, reservationTool)
agent.RegisterSkill(executableSkill)

// Load instruction skills (markdown-based)
agent.LoadSkillsFromDirectory("skills")

// Agent now has both types and uses them appropriately
```

The agent automatically separates them in prompts:
- **Instruction skills**: Full instructions included under "Expert Skills"
- **Executable skills**: Listed briefly under "Available Workflows"

## Future Enhancements

- [ ] Skill versioning and updates
- [ ] Skill dependencies and composition
- [ ] Dynamic skill selection based on context
- [ ] Skill performance metrics
- [ ] Community skill repository
