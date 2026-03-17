package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// MCPToolWrapper wraps an MCP tool as an agent.Tool
type MCPToolWrapper struct {
	client   *Client
	toolName string
	toolDef  *sdkmcp.Tool
}

// NewMCPToolWrapper creates a wrapper that implements agent.Tool for an MCP tool
func NewMCPToolWrapper(client *Client, toolName string) (agent.Tool, error) {
	toolDef, ok := client.GetTool(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %s not found in MCP server", toolName)
	}

	return &MCPToolWrapper{
		client:   client,
		toolName: toolName,
		toolDef:  toolDef,
	}, nil
}

// Name returns the tool name prefixed with "mcp_" to avoid conflicts
func (m *MCPToolWrapper) Name() string {
	return "mcp_" + m.toolName
}

// Description returns the tool description from the MCP server
func (m *MCPToolWrapper) Description() string {
	desc := m.toolDef.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool: %s", m.toolName)
	}
	return desc
}

// Execute runs the MCP tool with the given input
func (m *MCPToolWrapper) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	// Parse the input as a map
	var args map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal MCP tool arguments: %w", err)
		}
	}

	// Call the MCP tool
	result, err := m.client.CallTool(ctx, m.toolName, args)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from the result
	var output string
	if len(result.Content) > 0 {
		for _, content := range result.Content {
			switch c := content.(type) {
			case *sdkmcp.TextContent:
				output += c.Text
			case *sdkmcp.ImageContent:
				output += fmt.Sprintf("[Image: %s]", c.MIMEType)
			case *sdkmcp.EmbeddedResource:
				if c.Resource != nil {
					output += fmt.Sprintf("[Resource: %s]", c.Resource.URI)
				}
			}
		}
	}

	// Return the result as JSON
	response := map[string]any{
		"output":  output,
		"success": !result.IsError,
	}

	return json.Marshal(response)
}

// LoadMCPTools creates agent.Tool wrappers for all tools from an MCP client
func LoadMCPTools(client *Client) ([]agent.Tool, error) {
	toolNames := client.ListTools()
	tools := make([]agent.Tool, 0, len(toolNames))

	for _, name := range toolNames {
		tool, err := NewMCPToolWrapper(client, name)
		if err != nil {
			return nil, fmt.Errorf("failed to wrap tool %s: %w", name, err)
		}
		tools = append(tools, tool)
	}

	return tools, nil
}
