# MachineSpirit

MachineSpirit is a Go-based framework for building intelligent agents with LLM-powered reasoning, tool execution, and memory management.

## Features

### Core Components

- **LLM Integration**: Support for multiple LLM providers (OpenAI, Anthropic)
- **Stateful Sessions**: Maintain conversation history across multiple interactions
- **Agent System**: Multi-step reasoning with tool calling and memory
- **Skills Framework**: High-level, composable capabilities that extend tools
- **Memory Management**: Store and retrieve facts for context-aware decision-making
- **Tool Framework**: Extensible system for adding custom actions
- **Interactive CLI**: Ready-to-use command-line interface

### Agent Capabilities

The agent system implements a complete reasoning loop:

1. **Perception**: Receive and understand user input
2. **Memory Retrieval**: Search for relevant context from long-term storage
3. **Decision Making**: Use LLM reasoning to determine actions
4. **Action Execution**: Invoke tools to accomplish tasks
5. **Feedback Loop**: Process results and replan if needed

## Installation

```bash
go get github.com/wzshiming/MachineSpirit
```

## Quick Start

### Basic LLM Session

```go
package main

import (
    "context"
    "fmt"
    "github.com/wzshiming/MachineSpirit/pkg/llm"
)

func main() {
    // Create LLM provider
    provider, _ := llm.NewLLM(
        llm.WithProvider("openai"),
        llm.WithAPIKey("your-api-key"),
    )

    // Create session
    session := llm.NewSession(provider, llm.SessionConfig{
        SystemPrompt: "You are a helpful assistant.",
    })

    // Complete a prompt
    ctx := context.Background()
    response, _ := session.Complete(ctx, llm.Message{
        Role:    llm.RoleUser,
        Content: "Hello!",
    })

    fmt.Println(response.Content)
}
```

### Agent with Tools

```go
package main

import (
    "context"
    "fmt"
    "github.com/wzshiming/MachineSpirit/pkg/agent"
    "github.com/wzshiming/MachineSpirit/pkg/llm"
)

func main() {
    // Setup LLM and session
    provider, _ := llm.NewLLM(llm.WithProvider("openai"))
    session := llm.NewSession(provider, llm.SessionConfig{
        SystemPrompt: "You are a travel assistant.",
    })

    // Create agent with tools
    ag, _ := agent.NewAgent(agent.Config{
        Session: session,
        Tools: []agent.Tool{
            agent.NewFlightSearchTool(),
            agent.NewFlightReservationTool(),
        },
    })

    // Store user preferences
    ag.Memory().Store("preferred_airline", "Delta")

    // Execute agent task
    ctx := context.Background()
    response, _ := ag.Execute(ctx, "Book a flight from NYC to London")

    fmt.Println(response)
}
```

## CLI Tools

### Basic Chat CLI

```bash
go run ./cmd/ms -provider openai -api-key YOUR_KEY
```

### Agent CLI (with tools and skills)

```bash
go run ./cmd/agent -provider openai -api-key YOUR_KEY
```

The agent CLI demonstrates both low-level tools and high-level skills for flight booking.

#### CLI Commands

- `/help` - Show available commands
- `/reset` - Clear the session
- `/memory` - Display memory contents
- `/store key=value` - Store a fact
- `/bye` - Exit

## Architecture

### Package Structure

```
MachineSpirit/
├── cmd/
│   ├── ms/          # Basic CLI chat interface
│   └── agent/       # Agent CLI with tools
├── pkg/
│   ├── llm/         # LLM provider abstraction and sessions
│   └── agent/       # Agent system with tools, skills, and memory
└── docs/
    ├── agent.md     # Detailed agent documentation
    └── skills.md    # Skills system documentation
```

### LLM Package

- **LLM Interface**: Abstract LLM providers
- **Session**: Manage conversation state
- **Providers**: OpenAI and Anthropic implementations

### Agent Package

- **Agent**: Main orchestration logic with skill/tool routing
- **Tool Interface**: Extensible action system for low-level operations
- **Skill Interface**: High-level, composable capabilities
- **Memory**: Fact storage and retrieval
- **MultiToolInvoker**: Smart router between skills and tools
- **Example Tools**: Flight search and reservation
- **Example Skills**: End-to-end flight booking

## Creating Custom Tools

Implement the `Tool` interface:

```go
type MyTool struct{}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Description of what this tool does"
}

func (t *MyTool) ParametersSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param": map[string]interface{}{
                "type": "string",
            },
        },
    }
}

func (t *MyTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
    // Tool implementation
    return "result", nil
}
```

Register with agent:

```go
agent.RegisterTool(&MyTool{})
```

## Configuration

### Environment Variables

- `OPENAI_API_KEY`: OpenAI API key
- `ANTHROPIC_API_KEY`: Anthropic API key

### Provider Options

```go
llm.NewLLM(
    llm.WithProvider("openai"),      // or "anthropic"
    llm.WithModel("gpt-4"),           // optional
    llm.WithAPIKey("key"),            // optional if env var set
    llm.WithBaseURL("https://..."),  // optional custom endpoint
)
```

## Testing

Run all tests:

```bash
go test ./... -v
```

Run specific package tests:

```bash
go test ./pkg/agent/... -v
go test ./pkg/llm/... -v
```

## Example: Flight Booking

The system includes a complete flight booking example:

```
User: Help me book a flight from New York to London for tomorrow

Agent Process:
1. Retrieves user preferences from memory (preferred airline)
2. Calls flight_search tool to find available flights
3. Analyzes results (price, time, airline preference)
4. Calls flight_reservation tool to book the selected flight
5. Returns confirmation to user

If any step fails, the agent replans or asks for clarification.
```

## Documentation

- [Agent System Documentation](docs/agent.md) - Detailed agent architecture and usage
- [Skills System Documentation](docs/skills.md) - Skills framework and best practices

## Development

### Building

```bash
go build ./...
```

### Running Tests

```bash
go test ./...
```

### Code Structure

- Follow existing patterns and conventions
- Add tests for new functionality
- Keep components loosely coupled
- Use interfaces for extensibility

## License

MIT License - see [LICENSE](LICENSE) file for details

## Contributing

Contributions are welcome! Please:

1. Follow Go best practices
2. Add tests for new features
3. Update documentation
4. Ensure all tests pass

## Roadmap

- [ ] Streaming responses for long-running operations
- [ ] Parallel tool execution
- [ ] Persistent memory storage
- [ ] Additional tool examples
- [ ] Web API interface
- [ ] Advanced reasoning strategies (ReAct, Chain-of-Thought)
