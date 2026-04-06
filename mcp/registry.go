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
	"fmt"
	"github.com/osiloke/adk-go-essentials/observability"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

// Registry manages MCP server connections and provides toolsets to agents.
type Registry struct {
	// Config is the MCP configuration.
	Config *Config
	// clients holds active MCP client connections keyed by server name.
	clients map[string]*Client
}

// NewRegistry creates a new MCP registry with the given configuration.
func NewRegistry(config *Config) *Registry {
	return &Registry{
		Config:  config,
		clients: make(map[string]*Client),
	}
}

// InitMCPRegistry creates and initializes an MCP registry from environment configuration.
// This is the main entry point for setting up MCP support.
func InitMCPRegistry(ctx context.Context) (*Registry, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load MCP config: %w", err)
	}

	registry := NewRegistry(config)

	if len(config.Servers) == 0 {
		observability.Log.Debugf("[MCP Registry] No MCP servers configured")
		return registry, nil
	}

	// Initialize all configured MCP clients
	for name, serverConfig := range config.Servers {
		if !serverConfig.Enabled {
			continue
		}

		if err := registry.InitializeClient(ctx, name); err != nil {
			observability.Log.Warnf("[MCP Registry] Failed to initialize server %q: %v", name, err)
			// Continue with other servers
		}
	}

	observability.Log.Infof("[MCP Registry] Initialized %d MCP server(s)", len(registry.clients))
	return registry, nil
}

// InitializeClient creates and connects an MCP client for the specified server.
func (r *Registry) InitializeClient(ctx context.Context, name string) error {
	if _, exists := r.clients[name]; exists {
		return fmt.Errorf("client for server %q already initialized", name)
	}

	config, exists := r.Config.Servers[name]
	if !exists {
		return fmt.Errorf("server %q not found in configuration", name)
	}

	observability.Log.Debugf("[MCP Registry] Initializing client for server %q (%s)", name, config.Endpoint)

	client, err := NewClient(ctx, config)
	if err != nil {
		observability.Log.Errorf("[MCP Registry] Failed to create client for server %q: %v", name, err)
		return fmt.Errorf("failed to create client for server %q: %w", name, err)
	}

	if err := client.Connect(ctx); err != nil {
		observability.Log.Errorf("[MCP Registry] Failed to connect to server %q: %v", name, err)
		return fmt.Errorf("failed to connect to server %q: %w", name, err)
	}

	r.clients[name] = client
	observability.Log.Infof("[MCP Registry] Connected to MCP server %q (%s)", name, client.ServerInfo.Name)

	return nil
}

// GetClient returns the MCP client for the specified server.
func (r *Registry) GetClient(name string) (*Client, error) {
	client, exists := r.clients[name]
	if !exists {
		return nil, fmt.Errorf("client for server %q not found", name)
	}
	return client, nil
}

// GetToolset creates an ADK toolset for the specified MCP server.
// This toolset can be passed directly to ADK agents.
func (r *Registry) GetToolset(name string) (tool.Toolset, error) {
	client, err := r.GetClient(name)
	if err != nil {
		observability.Log.Errorf("[MCP Registry] Failed to get client for server %q: %v", name, err)
		return nil, err
	}

	observability.Log.Debugf("[MCP Registry] Creating toolset for server %q", name)

	// Use ADK's built-in MCP toolset wrapper
	toolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: client.Transport,
	})
	if err != nil {
		observability.Log.Errorf("[MCP Registry] Failed to create toolset for server %q: %v", name, err)
		return nil, fmt.Errorf("failed to create toolset for server %q: %w", name, err)
	}

	observability.Log.Debugf("[MCP Registry] Toolset created successfully for server %q", name)
	return toolset, nil
}

// GetDefaultToolset returns the toolset for the default MCP server.
func (r *Registry) GetDefaultToolset() (tool.Toolset, error) {
	if r.Config.DefaultServer == "" {
		return nil, fmt.Errorf("no default MCP server configured")
	}
	return r.GetToolset(r.Config.DefaultServer)
}

// GetAllToolsets returns toolsets for all connected MCP servers.
// Useful for agents that need access to all available MCP tools.
func (r *Registry) GetAllToolsets() ([]tool.Toolset, error) {
	var toolsets []tool.Toolset

	for name := range r.clients {
		ts, err := r.GetToolset(name)
		if err != nil {
			observability.Log.Warnf("[MCP Registry] Failed to get toolset for server %q: %v", name, err)
			continue
		}
		toolsets = append(toolsets, ts)
	}

	return toolsets, nil
}

// ListAvailableTools returns a list of all available tools across all connected MCP servers.
func (r *Registry) ListAvailableTools(ctx context.Context) (map[string][]string, error) {
	result := make(map[string][]string)

	for name, client := range r.clients {
		tools, err := client.ListTools(ctx)
		if err != nil {
			observability.Log.Warnf("[MCP Registry] Failed to list tools for server %q: %v", name, err)
			continue
		}

		var toolNames []string
		for _, t := range tools {
			toolNames = append(toolNames, t.Name)
		}
		result[name] = toolNames
	}

	return result, nil
}

// Close shuts down all MCP connections.
func (r *Registry) Close() error {
	var lastErr error
	for name, client := range r.clients {
		if err := client.Close(); err != nil {
			observability.Log.Warnf("[MCP Registry] Failed to close client %q: %v", name, err)
			lastErr = err
		}
	}
	r.clients = make(map[string]*Client)
	return lastErr
}

// GetServerInfo returns information about a connected MCP server.
func (r *Registry) GetServerInfo(name string) (*ServerInfo, error) {
	client, err := r.GetClient(name)
	if err != nil {
		return nil, err
	}

	if client.ServerInfo == nil {
		return nil, fmt.Errorf("server info not available for %q", name)
	}

	return &ServerInfo{
		Name:    client.ServerInfo.Name,
		Version: client.ServerInfo.Version,
	}, nil
}

// ServerInfo contains information about an MCP server.
type ServerInfo struct {
	Name    string
	Version string
}

// IsServerConnected returns true if the specified server is connected.
func (r *Registry) IsServerConnected(name string) bool {
	client, exists := r.clients[name]
	if !exists {
		return false
	}
	return client.IsConnected()
}
