# MCP (Model Context Protocol) Package

This package provides integration with MCP servers, allowing MachineSpirit to use tools from any MCP-compatible server.

## Overview

The Model Context Protocol (MCP) is a standardized protocol for connecting AI agents to external tools and resources. This package allows MachineSpirit to connect to MCP servers and use their tools seamlessly.

## Features

- **Automatic Tool Discovery**: Connects to MCP servers and automatically discovers available tools
- **Tool Wrapping**: Wraps MCP tools to implement the `agent.Tool` interface
- **Namespace Isolation**: Prefixes MCP tools with `mcp_` to avoid naming conflicts
- **Multiple Content Types**: Supports text, image, and embedded resource content from MCP tools

## Usage

### Command Line

Use the `--mcp-server` flag to connect to an MCP server:

```bash
# Run an MCP server directly
./ms --mcp-server "go run github.com/modelcontextprotocol/go-sdk/examples/server/hello"

# Or use a pre-built MCP server binary
./ms --mcp-server "/path/to/mcp-server arg1 arg2"
```

### Programmatic Usage

```go
import (
    "context"
    "github.com/wzshiming/MachineSpirit/pkg/mcp"
)

// Connect to an MCP server
client, err := mcp.NewClient(ctx, "server-command", "arg1", "arg2")
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// List available tools
tools := client.ListTools()
fmt.Printf("Available tools: %v\n", tools)

// Call a tool
result, err := client.CallTool(ctx, "tool-name", map[string]any{
    "arg1": "value1",
})

// Load all MCP tools as agent.Tool wrappers
agentTools, err := mcp.LoadMCPTools(client)
```

## Architecture

### Client (`client.go`)

The `Client` struct wraps the MCP SDK client and provides:
- Connection management to MCP servers via stdio transport
- Tool discovery and caching
- Tool execution with proper error handling

### MCPToolWrapper (`tool.go`)

The `MCPToolWrapper` adapts MCP tools to the `agent.Tool` interface:
- Implements `Name()`, `Description()`, and `Execute()` methods
- Handles JSON marshaling/unmarshaling
- Extracts content from MCP tool results (text, images, resources)
- Prefixes tool names with `mcp_` to avoid conflicts

## Examples

### Using the Hello Server

```bash
# Build the example hello server
cd /tmp/go-sdk/examples/server/hello
go build -o hello-server

# Use it with MachineSpirit
./ms --mcp-server "./hello-server"
```

Once connected, the agent can use the `mcp_greet` tool:

```
> Use the mcp_greet tool to say hi to Alice
```

### Creating a Custom MCP Server

Any MCP-compatible server can be used. See the [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) for examples of creating custom servers.

## Implementation Details

- **Transport**: Uses `CommandTransport` to spawn MCP servers as subprocesses
- **Protocol**: Implements MCP protocol via the official Go SDK
- **Threading**: Thread-safe client with mutex-protected tool cache
- **Error Handling**: Propagates MCP errors and handles tool execution failures

## References

- [MCP Specification](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Examples](https://github.com/modelcontextprotocol/go-sdk/tree/main/examples)
