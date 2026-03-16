# Subagent Support

MachineSpirit supports delegating specific tasks to **subagents**. A subagent is a specialized agent that focuses on a single task while the master agent can observe its progress and maintain human-in-the-loop control.

## Overview

The subagent feature allows you to:
- **Delegate specific tasks**: Offload focused work to a specialized agent
- **Observable execution**: Monitor what the subagent is doing in real-time
- **Human approval**: All subagent actions require human approval before execution
- **Isolated context**: Each subagent operates independently with its own session
- **Controlled tool access**: Specify which tools the subagent can use

## How It Works

### Architecture

1. **Master Agent**: The primary agent that coordinates tasks and can delegate work
2. **Subagent**: A specialized agent created for a specific task
3. **Observable Tools**: Tools wrapped with approval and observation capabilities
4. **Approval Function**: Requests human permission before each action
5. **Observer Function**: Reports subagent progress to the master agent

### Workflow

```
Master Agent
    ↓ (delegates task)
Subagent Created
    ↓ (requests tool use)
Human Approval
    ↓ (if approved)
Tool Execution
    ↓ (reports progress)
Observable Feedback
    ↓ (task complete)
Result Returned to Master
```

## Using the Subagent Tool

### From the CLI

When interacting with MachineSpirit, you can ask the master agent to delegate tasks to a subagent:

```
> I need you to analyze the logs in /var/log/app.log. Use a subagent for this task.
```

The agent will use the `subagent` tool with appropriate parameters:

```json
{
  "tool": "subagent",
  "input": {
    "task": "Analyze the logs in /var/log/app.log and report any errors",
    "description": "Look for error patterns and summarize findings",
    "tools": ["read", "bash"]
  }
}
```

### Human Approval Flow

When a subagent is created or attempts to use a tool, you'll see approval prompts:

```
⚠️  Subagent requesting approval:
Start subagent for task: Analyze the logs in /var/log/app.log and report any errors
Approve? (y/n): y
✓ Approved

  🤖 Starting subagent for task: Analyze the logs in /var/log/app.log and report any errors
  📋 Subagent is processing the task...

⚠️  Subagent requesting approval:
Tool: read, Input: {"file":"/var/log/app.log"}
Approve? (y/n): y
✓ Approved

  🔧 Subagent wants to use: read
  ⚙️  Executing: read
  ✓ Tool read completed
  ✅ Subagent completed the task
```

### Observable Progress

Throughout execution, you'll see real-time updates:
- 🤖 **Starting**: When the subagent begins
- 📋 **Processing**: Task execution in progress
- 🔧 **Tool Request**: When the subagent wants to use a tool
- ⚙️ **Executing**: Active tool execution
- ✓ **Completed**: Tool finished successfully
- ❌ **Failed**: Tool execution error
- ✅ **Done**: Subagent task completed

## Tool Input Format

The subagent tool accepts the following input:

```json
{
  "task": "string (required)",        // The specific task to delegate
  "description": "string (optional)", // Additional context
  "tools": ["array", "of", "names"]   // Tools the subagent can use (empty = all)
}
```

## Tool Output Format

The subagent tool returns:

```json
{
  "status": "success|failed|cancelled",
  "result": "string",                  // Final result from subagent
  "actions": ["array", "of", "actions"], // List of actions performed
  "error": "string (optional)",        // Error message if failed
  "cancelled": "string (optional)"     // Reason if cancelled
}
```

## Example Use Cases

### 1. Code Analysis

```
Master: "Create a subagent to analyze the Python code in src/ and report code quality issues"

Subagent receives:
- Task: "Analyze Python code in src/ for quality issues"
- Tools: ["read", "bash"]
```

### 2. Data Processing

```
Master: "Use a subagent to process the CSV file data.csv and calculate statistics"

Subagent receives:
- Task: "Process data.csv and calculate mean, median, and std deviation"
- Tools: ["read", "bash"]
```

### 3. File Operations

```
Master: "Delegate the task of organizing files in /tmp/downloads to a subagent"

Subagent receives:
- Task: "Organize files in /tmp/downloads by type into subdirectories"
- Tools: ["read", "write", "bash"]
```

## Programmatic Usage

### Creating a Subagent Tool

```go
import (
    "github.com/wzshiming/MachineSpirit/pkg/agent"
    "github.com/wzshiming/MachineSpirit/pkg/agent/tools"
    "github.com/wzshiming/MachineSpirit/pkg/llm"
    "github.com/wzshiming/MachineSpirit/pkg/persistence"
)

// Define approval function
approvalFunc := func(ctx context.Context, action string) (bool, error) {
    fmt.Printf("Approve: %s? (y/n): ", action)
    var response string
    fmt.Scanln(&response)
    return response == "y", nil
}

// Define observer function
observerFunc := func(message string) {
    fmt.Println("Subagent:", message)
}

// Create the tool
subagentTool := tools.NewSubAgentTool(
    llmProvider,
    persistenceManager,
    []agent.Tool{bashTool, readTool, writeTool},
    approvalFunc,
    observerFunc,
)

// Add to agent's tools
agent.WithTools(subagentTool)
```

### Custom Approval Logic

You can implement custom approval logic:

```go
// Auto-approve read-only operations
approvalFunc := func(ctx context.Context, action string) (bool, error) {
    if strings.Contains(action, `"tool": "read"`) {
        return true, nil // Auto-approve reads
    }
    // Require manual approval for other operations
    fmt.Printf("Approve: %s? (y/n): ", action)
    var response string
    fmt.Scanln(&response)
    return response == "y", nil
}
```

### Custom Observer

Implement custom observers for logging or UI updates:

```go
// Log to file
observerFunc := func(message string) {
    log.Printf("[Subagent] %s", message)
    // Could also send to UI, database, etc.
}
```

## Benefits

1. **Task Isolation**: Complex tasks can be handled independently without affecting the master agent's context
2. **Safety**: Human approval ensures no unintended actions are taken
3. **Transparency**: Observable execution provides visibility into what's happening
4. **Flexibility**: Tool access can be restricted per subagent
5. **Parallel Processing**: Multiple subagents could theoretically work on different tasks (future enhancement)

## Limitations

1. **Sequential Only**: Currently, subagents execute sequentially (no parallel subagents yet)
2. **No Subagent Nesting**: Subagents cannot create their own subagents
3. **Shared Tools**: Subagents use the same tool instances (no isolation at tool level)
4. **Memory**: Subagents don't share the master's conversation history

## Future Enhancements

- **Parallel Subagents**: Support multiple subagents working simultaneously
- **Subagent Communication**: Allow subagents to share information
- **Persistent Subagents**: Long-running subagents that can be resumed
- **Skill Support**: Allow subagents to use skills
- **Custom System Prompts**: Different personality/instructions per subagent

## Security Considerations

- Always review approval requests carefully before accepting
- Restrict tool access based on the subagent's task
- Monitor observable output for unexpected behavior
- Subagents inherit the same file system access as the master agent
- Consider running subagents in sandboxed environments for untrusted tasks

## See Also

- [Agent Architecture](../pkg/agent/agent.go)
- [Tool Interface](../pkg/agent/tool.go)
- [Subagent Implementation](../pkg/agent/tools/subagent.go)
- [Subagent Tests](../pkg/agent/tools/subagent_test.go)
