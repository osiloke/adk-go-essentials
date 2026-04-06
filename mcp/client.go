// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/osiloke/adk-go-essentials/observability"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// Client wraps an MCP client connection with configuration and lifecycle management.
type Client struct {
	// Config is the server configuration used to create this client.
	Config *ServerConfig
	// Transport is the underlying MCP transport.
	Transport mcp.Transport
	// Client is the MCP client.
	Client *mcp.Client
	// Session is the connected MCP client session.
	Session *mcp.ClientSession
	// ServerInfo contains information about the connected MCP server.
	ServerInfo *mcp.Implementation
}

// NewClient creates a new MCP client based on the provided configuration.
func NewClient(ctx context.Context, config *ServerConfig) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	var transport mcp.Transport
	var err error

	switch config.Type {
	case ServerTypeLocal:
		transport, err = createLocalTransport(ctx, config)
	case ServerTypeRemote:
		transport, err = createRemoteTransport(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported server type: %v", config.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	return &Client{
		Config:    config,
		Transport: transport,
	}, nil
}

// createLocalTransport creates an in-memory MCP transport for local servers.
func createLocalTransport(ctx context.Context, config *ServerConfig) (mcp.Transport, error) {
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Create and start the in-memory MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    config.Name,
		Version: "1.0.0",
	}, nil)

	// Register built-in tools based on configuration
	for _, toolName := range config.Tools {
		if err := registerBuiltinTool(server, toolName); err != nil {
			log.Printf("Warning: failed to register tool %q: %v", toolName, err)
		}
	}

	// Connect the server to the transport
	_, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect local server: %w", err)
	}

	return clientTransport, nil
}

// createRemoteTransport creates a remote MCP transport (HTTP/SSE).
func createRemoteTransport(ctx context.Context, config *ServerConfig) (mcp.Transport, error) {
	observability.Log.Debugf("[MCP Client] Creating remote transport for server %q", config.Name)
	observability.Log.Debugf("[MCP Client] Endpoint URL: %s", config.Endpoint)
	observability.Log.Debugf("[MCP Client] Auth Type: %s, Token Env: %s", config.AuthType, config.TokenEnv)

	switch config.AuthType {
	case AuthTypeOAuth2:
		token := config.GetAuthToken()
		if token == "" {
			return nil, fmt.Errorf("OAuth2 token is empty")
		}
		observability.Log.Debugf("[MCP Client] Using OAuth2 authentication for server %q", config.Name)

		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		baseHTTPClient := oauth2.NewClient(ctx, ts)

		// Wrap with resilient transport
		httpClient := &http.Client{
			Timeout:   DefaultHTTPTimeout,
			Transport: baseHTTPClient.Transport,
		}
		resilientClient := wrapWithResilientTransport(httpClient)

		return &mcp.StreamableClientTransport{
			Endpoint:   config.Endpoint,
			HTTPClient: resilientClient,
		}, nil

	case AuthTypeAPIKey:
		token := config.GetAuthToken()
		if token == "" {
			return nil, fmt.Errorf("API key token is empty")
		}
		observability.Log.Debugf("[MCP Client] Using API Key authentication for server %q", config.Name)

		// For API key auth, create a resilient HTTP client with API key authentication
		httpConfig := DefaultHTTPClientConfig()
		apiKeyClient := &http.Client{
			Timeout:   httpConfig.Timeout,
			Transport: newAPIKeyTransport(token, config.Endpoint, httpConfig),
		}

		return &mcp.StreamableClientTransport{
			Endpoint:   config.Endpoint,
			HTTPClient: apiKeyClient,
		}, nil

	case AuthTypeNone:
		observability.Log.Debugf("[MCP Client] Using no authentication for server %q", config.Name)
		return &mcp.StreamableClientTransport{
			Endpoint: config.Endpoint,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported auth type: %v", config.AuthType)
	}
}

// wrapWithResilientTransport wraps an existing HTTP client's transport with retry logic.
func wrapWithResilientTransport(client *http.Client) *http.Client {
	config := DefaultHTTPClientConfig()
	client.Transport = newRetryableTransport(config)
	client.Timeout = config.Timeout
	return client
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Connect establishes the MCP session and retrieves server capabilities.
func (c *Client) Connect(ctx context.Context) error {
	if c.Session != nil {
		return fmt.Errorf("client is already connected")
	}

	observability.Log.Debugf("[MCP Client] Connecting to server %q at %s", c.Config.Name, c.Config.Endpoint)

	// Create MCP client
	c.Client = mcp.NewClient(&mcp.Implementation{
		Name:    "product-designer",
		Version: "1.0.0",
	}, nil)

	// Connect with retry logic
	var session *mcp.ClientSession
	var err error

	maxAttempts := 3
	baseDelay := 500 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		session, err = c.Client.Connect(ctx, c.Transport, nil)
		if err == nil {
			break
		}

		observability.Log.Debugf("[MCP Client] Connect attempt %d/%d failed: %v", attempt, maxAttempts, err)

		// Don't retry on last attempt
		if attempt == maxAttempts {
			observability.Log.Errorf("[MCP Client] Failed to connect to server %q after %d attempts: %v", c.Config.Name, maxAttempts, err)
			return fmt.Errorf("failed to connect after %d attempts: %w", maxAttempts, err)
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			observability.Log.Errorf("[MCP Client] Non-retryable error connecting to server %q: %v", c.Config.Name, err)
			return fmt.Errorf("failed to connect: %w", err)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(baseDelay):
			baseDelay *= 2 // Exponential backoff
		}
	}

	observability.Log.Infof("[MCP Client] Successfully connected to server %q", c.Config.Name)

	c.Session = session

	// Get server info from the initialize result
	initResult := session.InitializeResult()
	if initResult != nil {
		c.ServerInfo = initResult.ServerInfo
		observability.Log.Debugf("[MCP Client] Server %q version: %s", c.ServerInfo.Name, c.ServerInfo.Version)
	}

	return nil
}

// Close closes the MCP session and underlying transport.
func (c *Client) Close() error {
	if c.Session != nil {
		err := c.Session.Close()
		c.Session = nil
		return err
	}
	return nil
}

// IsConnected returns true if the client has an active session.
func (c *Client) IsConnected() bool {
	return c.Session != nil
}

// ListTools retrieves the list of available tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	if c.Session == nil {
		return nil, fmt.Errorf("client is not connected")
	}

	// List tools with retry logic
	var result *mcp.ListToolsResult
	var err error

	maxAttempts := 3
	baseDelay := 200 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err = c.Session.ListTools(ctx, nil)
		if err == nil {
			break
		}

		observability.Log.Debugf("[MCP Client] ListTools attempt %d/%d failed: %v", attempt, maxAttempts, err)

		// Don't retry on last attempt
		if attempt == maxAttempts {
			return nil, fmt.Errorf("failed to list tools after %d attempts: %w", maxAttempts, err)
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(baseDelay):
			baseDelay *= 2 // Exponential backoff
		}
	}

	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	if c.Session == nil {
		observability.Log.Errorf("[MCP Client] Cannot call tool %q: client is not connected", name)
		return nil, fmt.Errorf("client is not connected")
	}

	observability.Log.Debugf("[MCP Client] Calling tool %q on server %q", name, c.Config.Name)

	// Special handling for list_projects - log for debugging
	if name == "list_projects" || name == "stitch_list_projects" {
		observability.Log.Infof("[MCP Client] >>> CALLING list_projects directly through MCP client")
	}

	params := &mcp.CallToolParams{
		Name: name,
	}

	// Only marshal arguments if provided (nil args means no arguments)
	if args != nil {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			observability.Log.Errorf("[MCP Client] Failed to marshal arguments for tool %q: %v", name, err)
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}
		params.Arguments = argsJSON
	}

	// Call tool with retry logic
	var result *mcp.CallToolResult
	var err error

	maxAttempts := 3
	baseDelay := 200 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err = c.Session.CallTool(ctx, params)
		if err == nil {
			break
		}

		observability.Log.Debugf("[MCP Client] Tool %q call attempt %d/%d failed: %v", name, attempt, maxAttempts, err)

		// Don't retry on last attempt
		if attempt == maxAttempts {
			observability.Log.Errorf("[MCP Client] Tool %q call failed after %d attempts: %v", name, maxAttempts, err)
			return nil, fmt.Errorf("tool call failed after %d attempts: %w", maxAttempts, err)
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			observability.Log.Errorf("[MCP Client] Tool %q call failed with non-retryable error: %v", name, err)
			return nil, fmt.Errorf("tool call failed: %w", err)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(baseDelay):
			baseDelay *= 2 // Exponential backoff
		}
	}

	observability.Log.Debugf("[MCP Client] Tool %q call succeeded", name)
	return result, nil
}

// registerBuiltinTool registers a built-in tool handler for local MCP servers.
// This is a placeholder - actual tool registration would depend on the specific tools needed.
func registerBuiltinTool(server *mcp.Server, toolName string) error {
	// Placeholder for built-in tool registration
	// In practice, you would have a registry of tool handlers
	switch toolName {
	case "get_weather":
		// Example tool registration using the generic AddTool
		// mcp.AddTool(server, &mcp.Tool{Name: "get_weather", Description: "..."}, GetWeatherHandler)
		return nil
	case "file_ops":
		// Register file operation tools
		return nil
	default:
		return fmt.Errorf("unknown built-in tool: %s", toolName)
	}
}
