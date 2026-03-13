# Skills System

The Skills system provides a high-level, composable capability framework that works alongside the existing Tools system. Skills are designed to handle complex, multi-step operations while Tools remain focused on low-level primitives.

## Overview

The MachineSpirit agent system now supports a layered architecture:

```
┌─────────────────────────────────────┐
│         Agent Loop                  │
└─────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│     MultiToolInvoker (Router)       │
├─────────────────┬───────────────────┤
│   Skills        │   Tools           │
│  (High-level)   │   (Low-level)     │
└─────────────────┴───────────────────┘
```

## Skills vs Tools

| Aspect | **Tools** | **Skills** |
|--------|-----------|------------|
| **Purpose** | Low-level operations | High-level capabilities |
| **Complexity** | Single-step | Multi-step, composable |
| **Documentation** | Basic description | Detailed description with examples |
| **Composition** | Standalone | Can compose multiple tools |
| **Use Case** | Direct API calls, simple operations | End-to-end workflows |

## Architecture Components

### 1. Skill Interface

```go
type Skill interface {
    Name() string
    Description() string
    DetailedDescription() string  // Rich documentation
    ParametersSchema() map[string]interface{}
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}
```

### 2. SkillRegistry

Manages skill discovery and registration:

```go
registry := agent.NewSkillRegistry()
registry.Register(mySkill)
skill, _ := registry.Get("skill_name")
```

### 3. MultiToolInvoker

Routes requests to appropriate handler (skill or tool):

```go
invoker := agent.NewMultiToolInvoker(tools, skillRegistry)

// Auto-detect and invoke
kind, output, err := invoker.InvokeAuto(ctx, "action_name", input)

// Explicit invocation
output, err := invoker.Invoke(ctx, agent.ToolKindSkill, "skill_name", input)
```

## Creating a Skill

### Example: Flight Booking Skill

```go
type FlightBookingSkill struct {
    searchTool      agent.Tool
    reservationTool agent.Tool
}

func NewFlightBookingSkill(searchTool, reservationTool agent.Tool) *FlightBookingSkill {
    return &FlightBookingSkill{
        searchTool:      searchTool,
        reservationTool: reservationTool,
    }
}

func (s *FlightBookingSkill) Name() string {
    return "flight_booking"
}

func (s *FlightBookingSkill) Description() string {
    return "End-to-end flight booking with intelligent flight selection"
}

func (s *FlightBookingSkill) DetailedDescription() string {
    return `Handles complete flight booking workflow:
1. Searches for available flights
2. Analyzes options based on preferences
3. Automatically selects best flight
4. Makes reservation

Composes flight_search and flight_reservation tools with
intelligent decision-making logic.`
}

func (s *FlightBookingSkill) ParametersSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "from":              {"type": "string"},
            "to":                {"type": "string"},
            "date":              {"type": "string"},
            "passenger_name":    {"type": "string"},
            "preferred_airline": {"type": "string"},
        },
        "required": []string{"from", "to", "date", "passenger_name"},
    }
}

func (s *FlightBookingSkill) Execute(ctx context.Context, input json.RawMessage) (string, error) {
    // 1. Parse input
    var params struct {
        From             string `json:"from"`
        To               string `json:"to"`
        Date             string `json:"date"`
        PassengerName    string `json:"passenger_name"`
        PreferredAirline string `json:"preferred_airline"`
    }
    json.Unmarshal(input, &params)

    // 2. Search flights
    searchInput, _ := json.Marshal(map[string]string{
        "from": params.From,
        "to":   params.To,
        "date": params.Date,
    })
    searchResult, err := s.searchTool.Execute(ctx, searchInput)
    if err != nil {
        return "", err
    }

    // 3. Select best flight (intelligent logic here)
    selectedFlight := selectBestFlight(searchResult, params.PreferredAirline)

    // 4. Make reservation
    reservationInput, _ := json.Marshal(map[string]string{
        "flight_number":  selectedFlight,
        "passenger_name": params.PassengerName,
    })
    return s.reservationTool.Execute(ctx, reservationInput)
}
```

## Using Skills with Agents

### Basic Setup

```go
// Create tools
searchTool := agent.NewFlightSearchTool()
reservationTool := agent.NewFlightReservationTool()

// Create skills registry
skills := agent.NewSkillRegistry()
bookingSkill := agent.NewFlightBookingSkill(searchTool, reservationTool)
skills.Register(bookingSkill)

// Create agent with both tools and skills
ag, _ := agent.NewAgent(agent.Config{
    Session: session,
    Tools: []agent.Tool{searchTool, reservationTool},
    Skills:  skills,
})
```

### Dynamic Registration

```go
// Register skill at runtime
customSkill := &MyCustomSkill{}
ag.RegisterSkill(customSkill)

// Register tool at runtime
customTool := &MyCustomTool{}
ag.RegisterTool(customTool)
```

## Wrapping Tools as Skills

Convert existing tools to skills for consistent interface:

```go
tool := agent.NewFlightSearchTool()
skill := agent.NewToolAsSkill(tool)

registry.Register(skill)
```

## How the Agent Uses Skills

When an agent receives a request:

1. **Prompt Construction**: Agent lists both skills and tools
   ```
   ## Available Skills (High-level capabilities):
   - flight_booking: End-to-end booking...

   ## Available Tools (Low-level operations):
   - flight_search: Search for flights...
   - flight_reservation: Reserve a flight...
   ```

2. **LLM Decision**: Agent chooses appropriate action
   ```xml
   <tool_call>{"tool_name": "flight_booking", "input": {...}}</tool_call>
   ```

3. **Routing**: MultiToolInvoker automatically routes to skill or tool

4. **Execution**: Skill composes multiple tools internally

5. **Feedback**: Results returned to agent for next step

## Best Practices

### When to Use Skills

- **Complex Workflows**: Multi-step operations with decision logic
- **Composition**: Combining multiple tools
- **Business Logic**: Domain-specific intelligence
- **User-Facing Features**: High-level capabilities users request

### When to Use Tools

- **Primitives**: Basic operations (API calls, data transforms)
- **Building Blocks**: Components skills compose
- **Simple Operations**: Single-step actions
- **External Integrations**: Direct service calls

### Design Guidelines

1. **Skills should be task-oriented** ("book flight" not "search then reserve")
2. **Tools should be action-oriented** ("search flights", "reserve flight")
3. **Skills can call multiple tools** but tools should not call skills
4. **Prefer skills for user-facing operations**
5. **Keep tools focused and reusable**

## Testing

```go
func TestFlightBookingSkill(t *testing.T) {
    searchTool := agent.NewFlightSearchTool()
    reservationTool := agent.NewFlightReservationTool()
    skill := agent.NewFlightBookingSkill(searchTool, reservationTool)

    input := json.RawMessage(`{
        "from": "NYC",
        "to": "LON",
        "date": "2026-03-15",
        "passenger_name": "John Doe",
        "preferred_airline": "Delta"
    }`)

    output, err := skill.Execute(context.Background(), input)
    // Assert output contains confirmation
}
```

## Future Enhancements

- **Markdown-based skills**: Load skills from configuration files
- **Skill composition**: Skills that compose other skills
- **Skill discovery**: Auto-discover skills from directories
- **MCP integration**: Model Context Protocol tool support
- **Skill validation**: Schema validation for parameters
- **Skill versioning**: Multiple versions of same skill

## Migration from Tools-Only

Existing tools-only code continues to work:

```go
// Old code (still works)
ag, _ := agent.NewAgent(agent.Config{
    Session: session,
    Tools:   []agent.Tool{tool1, tool2},
})

// New code (with skills)
ag, _ := agent.NewAgent(agent.Config{
    Session: session,
    Tools:   []agent.Tool{tool1, tool2},
    Skills:  skillRegistry,
})
```

The agent automatically routes to the correct handler based on the name used in tool calls.

## API Reference

### Agent Methods

- `RegisterSkill(skill Skill) error` - Add skill at runtime
- `GetSkillRegistry() *SkillRegistry` - Access skill registry
- `GetInvoker() *MultiToolInvoker` - Access router

### SkillRegistry Methods

- `Register(skill Skill) error` - Add skill
- `Unregister(name string) error` - Remove skill
- `Get(name string) (Skill, error)` - Retrieve skill
- `Has(name string) bool` - Check if skill exists
- `List() []Skill` - Get all skills
- `Count() int` - Number of registered skills
- `Clear()` - Remove all skills

### MultiToolInvoker Methods

- `InvokeAuto(ctx, name, input) (kind, output, error)` - Auto-detect and invoke
- `Invoke(ctx, kind, name, input) (output, error)` - Explicit invocation
- `Has(name string) bool` - Check if action exists
- `ListAll() map[ToolKind][]string` - List all actions by type
