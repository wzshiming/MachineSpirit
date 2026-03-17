package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client wraps an MCP client and session for convenient access to MCP server tools
type Client struct {
	mcpClient   *mcp.Client
	session     *mcp.ClientSession
	mu          sync.RWMutex
	tools       map[string]*mcp.Tool
	initialized bool
}

// NewClient creates a new MCP client and connects to an MCP server via a command
func NewClient(ctx context.Context, command string, args ...string) (*Client, error) {
	// Create the MCP client
	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "MachineSpirit",
		Version: "v1.0.0",
	}, nil)

	// Create a command transport to run the MCP server
	cmd := exec.Command(command, args...)
	transport := &mcp.CommandTransport{Command: cmd}

	// Connect to the server
	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	client := &Client{
		mcpClient: mcpClient,
		session:   session,
		tools:     make(map[string]*mcp.Tool),
	}

	// Initialize tools
	if err := client.loadTools(ctx); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to load tools: %w", err)
	}

	return client, nil
}

// loadTools fetches all available tools from the MCP server
func (c *Client) loadTools(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the server supports tools
	caps := c.session.InitializeResult().Capabilities
	if caps.Tools == nil {
		slog.Warn("MCP server does not support tools")
		c.initialized = true
		return nil
	}

	// Iterate through all available tools
	for tool, err := range c.session.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("error listing tools: %w", err)
		}
		c.tools[tool.Name] = tool
		slog.Debug("Loaded MCP tool", "name", tool.Name, "description", tool.Description)
	}

	c.initialized = true
	slog.Info("Loaded MCP tools", "count", len(c.tools))
	return nil
}

// ListTools returns a list of all available tool names
func (c *Client) ListTools() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.tools))
	for name := range c.tools {
		names = append(names, name)
	}
	return names
}

// GetTool returns the tool definition for a given tool name
func (c *Client) GetTool(name string) (*mcp.Tool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tool, ok := c.tools[name]
	return tool, ok
}

// CallTool executes a tool on the MCP server
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("MCP client not initialized")
	}

	params := &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := c.session.CallTool(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	if result.IsError {
		return result, fmt.Errorf("tool execution error: %s", name)
	}

	return result, nil
}

// Close closes the MCP client session
func (c *Client) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
