# Agent System

The agent system extends MachineSpirit with intelligent, multi-step reasoning capabilities. Agents can perceive user input, retrieve relevant context from memory, make decisions using LLM reasoning, execute tools, and handle feedback loops for replanning.

## Architecture

The agent system consists of four main components:

### 1. Agent (`agent.Agent`)

The core orchestrator that manages the agent loop:
- **Perception**: Receives and processes user input
- **Memory Retrieval**: Searches for relevant context from long-term memory
- **Decision Making**: Uses LLM reasoning to determine actions
- **Action Execution**: Invokes tools based on LLM decisions
- **Feedback Loop**: Processes results and replans if needed

### 2. Tools (`agent.Tool`)

Actions that agents can invoke to interact with external systems:
```go
type Tool interface {
    Name() string
    Description() string
    ParametersSchema() map[string]interface{}
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}
```

### 3. Memory (`agent.Memory`)

Storage for facts and context that inform agent decisions:
```go
type Memory interface {
    Store(key, value string)
    Retrieve(key string) string
    Search(query string) []Fact
    All() []Fact
    Clear()
}
```

### 4. Session Integration

Agents wrap an `llm.Session` to maintain conversation state while adding tool-calling capabilities.

## Usage Example

### Basic Agent Setup

```go
import (
    "context"
    "github.com/wzshiming/MachineSpirit/pkg/agent"
    "github.com/wzshiming/MachineSpirit/pkg/llm"
)

// Create LLM provider
provider, _ := llm.NewLLM(
    llm.WithProvider("openai"),
    llm.WithModel("gpt-4"),
)

// Create session
session := llm.NewSession(provider, llm.SessionConfig{
    SystemPrompt: "You are a helpful assistant with tool access.",
})

// Create agent with tools
ag, _ := agent.NewAgent(agent.Config{
    Session: session,
    Tools: []agent.Tool{
        agent.NewFlightSearchTool(),
        agent.NewFlightReservationTool(),
    },
})

// Store preferences in memory
ag.Memory().Store("preferred_airline", "Delta Airlines")

// Execute agent task
ctx := context.Background()
response, _ := ag.Execute(ctx, "Book a flight from New York to London for tomorrow")
```

### Flight Booking Example

The system includes example tools for flight booking:

```go
// Flight Search Tool
searchTool := agent.NewFlightSearchTool()
// Searches for flights between cities on a specific date
// Returns: list of flights with prices and times

// Flight Reservation Tool
reservationTool := agent.NewFlightReservationTool()
// Reserves a specific flight
// Returns: confirmation number if successful
```

### Complete Workflow

When a user says: "Help me book a flight ticket from New York to London for tomorrow"

1. **Perception**: Agent receives the user input

2. **Memory Retrieval**: Agent searches memory for relevant facts:
   - `preferred_airline: Delta Airlines`
   - `user_name: John Doe`

3. **Decision Making**: LLM reasons about the task:
   - Determines a flight search is needed
   - Generates tool call: `<tool_call>{"tool_name": "flight_search", "input": {...}}</tool_call>`

4. **Action**: Agent executes the `flight_search` tool:
   - Returns available flights with prices and times

5. **Continued Reasoning**: LLM analyzes results:
   - Compares prices and times
   - Considers user preferences (Delta Airlines)
   - Decides on the best flight

6. **Action**: Agent executes the `flight_reservation` tool:
   - Makes the reservation
   - Returns confirmation number

7. **Feedback**:
   - **Success**: Agent informs user with confirmation details
   - **Failure**: Agent replans or asks user for clarification

## Tool Calling Format

Agents use XML-style tags for tool calls:

```
<tool_call>{"tool_name": "tool_name", "input": {...}}</tool_call>
```

Multiple tool calls can be made in a single response:
```
<tool_call>{"tool_name": "flight_search", "input": {...}}</tool_call>
<tool_call>{"tool_name": "weather_check", "input": {...}}</tool_call>
```

## Creating Custom Tools

Implement the `Tool` interface:

```go
type MyCustomTool struct{}

func (t *MyCustomTool) Name() string {
    return "my_tool"
}

func (t *MyCustomTool) Description() string {
    return "What this tool does"
}

func (t *MyCustomTool) ParametersSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]interface{}{
                "type": "string",
                "description": "Parameter description",
            },
        },
        "required": []string{"param1"},
    }
}

func (t *MyCustomTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
    var params struct {
        Param1 string `json:"param1"`
    }
    if err := json.Unmarshal(input, &params); err != nil {
        return "", err
    }

    // Execute tool logic
    result := doSomething(params.Param1)

    return result, nil
}
```

Register the tool with an agent:
```go
agent.RegisterTool(&MyCustomTool{})
```

## Memory Management

### Storing Facts
```go
agent.Memory().Store("user_preference", "value")
```

### Retrieving Facts
```go
value := agent.Memory().Retrieve("user_preference")
```

### Searching Facts
```go
facts := agent.Memory().Search("preference")
```

### Clearing Memory
```go
agent.Memory().Clear()
```

## Error Handling and Retries

Agents automatically handle tool failures with retry logic:
- **Default**: Up to 3 retry attempts
- **Configurable**: Set `MaxRetries` in `agent.Config`
- **Feedback Loop**: On failure, LLM is informed and can replan

Example with custom retry limit:
```go
agent, _ := agent.NewAgent(agent.Config{
    Session:    session,
    Tools:      tools,
    MaxRetries: 5,
})
```

## CLI Usage

A dedicated CLI tool demonstrates agent capabilities:

```bash
# Build and run agent CLI
go build -o agent ./cmd/agent

# Start with OpenAI
./agent -provider openai -api-key YOUR_KEY

# Start with Anthropic
./agent -provider anthropic -api-key YOUR_KEY
```

### CLI Commands

- `/help` - Show available commands
- `/reset` - Clear the conversation session
- `/memory` - Display all facts in memory
- `/store key=value` - Store a fact in memory
- `/bye` - Exit the program

### Example Interaction

```
> Help me book a flight from New York to London for tomorrow
[Agent searches memory for preferences...]
[Agent calls flight_search tool...]
[Agent analyzes results...]
[Agent calls flight_reservation tool...]
Your flight has been booked! Confirmation: CONF-12345
```

## Testing

The agent system includes comprehensive tests:

```bash
go test ./pkg/agent/... -v
```

Tests cover:
- Agent initialization
- Tool registration and execution
- Memory operations
- Tool call parsing
- Feedback loops
- Error handling and retries

## Integration with Existing Code

The agent system builds on existing MachineSpirit components:
- Uses `llm.Session` for conversation management
- Compatible with all LLM providers (OpenAI, Anthropic)
- Follows existing patterns and conventions

## Future Enhancements

Potential improvements:
- Streaming responses for long-running tools
- Parallel tool execution
- Persistent memory storage (database/file)
- Tool result caching
- Agent-to-agent communication
- Advanced reasoning strategies (ReAct, Chain-of-Thought)
