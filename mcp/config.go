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

// Package mcp provides MCP (Model Context Protocol) client configuration and management.
// It supports both local (in-memory) and remote MCP servers with various authentication methods.
package mcp

import (
	"fmt"
	"os"
	"strings"
)

// ServerType represents the type of MCP server connection.
type ServerType string

const (
	// ServerTypeLocal represents an in-memory MCP server.
	ServerTypeLocal ServerType = "local"
	// ServerTypeRemote represents a remote MCP server (HTTP/SSE).
	ServerTypeRemote ServerType = "remote"
)

// AuthType represents the authentication method for MCP servers.
type AuthType string

const (
	// AuthTypeOAuth2 represents OAuth2 token-based authentication.
	AuthTypeOAuth2 AuthType = "oauth2"
	// AuthTypeAPIKey represents API key-based authentication.
	AuthTypeAPIKey AuthType = "api_key"
	// AuthTypeNone represents no authentication.
	AuthTypeNone AuthType = "none"
)

// ServerConfig holds the configuration for a single MCP server.
type ServerConfig struct {
	// Name is the unique identifier for this MCP server.
	Name string `json:"name"`
	// Type is the server connection type (local or remote).
	Type ServerType `json:"type"`
	// Endpoint is the URL for remote MCP servers (e.g., https://stitch.mcp.google.com).
	Endpoint string `json:"endpoint,omitempty"`
	// AuthType is the authentication method.
	AuthType AuthType `json:"auth_type,omitempty"`
	// TokenEnv is the environment variable name containing the auth token.
	TokenEnv string `json:"token_env,omitempty"`
	// Tools is a comma-separated list of tool names for local MCP servers.
	Tools []string `json:"tools,omitempty"`
	// Enabled indicates if this server should be initialized.
	Enabled bool `json:"enabled"`
}

// Config holds the configuration for all MCP servers.
type Config struct {
	// Servers is a map of server name to server configuration.
	Servers map[string]*ServerConfig `json:"servers"`
	// DefaultServer is the name of the default server to use.
	DefaultServer string `json:"default_server,omitempty"`
}

// LoadConfig loads MCP configuration from environment variables.
//
// Environment variables:
//   - MCP_SERVERS: Comma-separated list of server names (e.g., "stitch,github")
//   - MCP_<NAME>_TYPE: Server type ("local" or "remote")
//   - MCP_<NAME>_ENDPOINT: Remote server endpoint URL
//   - MCP_<NAME>_AUTH_TYPE: Auth type ("oauth2", "api_key", "none")
//   - MCP_<NAME>_TOKEN_ENV: Env var name containing the auth token
//   - MCP_<NAME>_TOOLS: Comma-separated list of tools (local servers only)
//   - MCP_DEFAULT_SERVER: Default server name (optional)
func LoadConfig() (*Config, error) {
	config := &Config{
		Servers: make(map[string]*ServerConfig),
	}

	// Get list of configured servers
	serversEnv := os.Getenv("MCP_SERVERS")
	if serversEnv == "" {
		// No MCP servers configured, return empty config
		return config, nil
	}

	serverNames := parseCommaSeparated(serversEnv)
	for _, name := range serverNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		serverConfig, err := loadServerConfig(name)
		if err != nil {
			return nil, fmt.Errorf("failed to load config for MCP server %q: %w", name, err)
		}
		config.Servers[name] = serverConfig
	}

	// Get default server
	config.DefaultServer = os.Getenv("MCP_DEFAULT_SERVER")
	if config.DefaultServer == "" && len(config.Servers) > 0 {
		// Use first server as default
		for name := range config.Servers {
			config.DefaultServer = name
			break
		}
	}

	return config, nil
}

// loadServerConfig loads configuration for a single MCP server from environment variables.
func loadServerConfig(name string) (*ServerConfig, error) {
	prefix := fmt.Sprintf("MCP_%s_", strings.ToUpper(name))

	// Get server type
	typeEnv := os.Getenv(prefix + "TYPE")
	if typeEnv == "" {
		return nil, fmt.Errorf("missing required env var: %sTYPE", prefix)
	}

	var serverType ServerType
	switch strings.ToLower(typeEnv) {
	case "local":
		serverType = ServerTypeLocal
	case "remote":
		serverType = ServerTypeRemote
	default:
		return nil, fmt.Errorf("invalid server type %q for server %q (must be 'local' or 'remote')", typeEnv, name)
	}

	// Get endpoint (required for remote servers)
	endpoint := os.Getenv(prefix + "ENDPOINT")
	if serverType == ServerTypeRemote && endpoint == "" {
		return nil, fmt.Errorf("missing required env var for remote server: %sENDPOINT", prefix)
	}

	// Get auth type (defaults to "none")
	authTypeEnv := os.Getenv(prefix + "AUTH_TYPE")
	if authTypeEnv == "" {
		authTypeEnv = "none"
	}

	var authType AuthType
	switch strings.ToLower(authTypeEnv) {
	case "oauth2":
		authType = AuthTypeOAuth2
	case "api_key":
		authType = AuthTypeAPIKey
	case "none":
		authType = AuthTypeNone
	default:
		return nil, fmt.Errorf("invalid auth type %q for server %q (must be 'oauth2', 'api_key', or 'none')", authTypeEnv, name)
	}

	// Get token environment variable (required for oauth2 and api_key)
	tokenEnv := os.Getenv(prefix + "TOKEN_ENV")
	if authType != AuthTypeNone && tokenEnv == "" {
		return nil, fmt.Errorf("missing required env var for %s auth: %sTOKEN_ENV", authType, prefix)
	}

	// Verify token env var exists
	if tokenEnv != "" && os.Getenv(tokenEnv) == "" {
		return nil, fmt.Errorf("token environment variable %q is not set (required by MCP server %q)", tokenEnv, name)
	}

	// Get tools list (for local servers)
	var tools []string
	toolsEnv := os.Getenv(prefix + "TOOLS")
	if toolsEnv != "" {
		tools = parseCommaSeparated(toolsEnv)
	}

	return &ServerConfig{
		Name:      name,
		Type:      serverType,
		Endpoint:  endpoint,
		AuthType:  authType,
		TokenEnv:  tokenEnv,
		Tools:     tools,
		Enabled:   true,
	}, nil
}

// parseCommaSeparated parses a comma-separated string into a slice of strings.
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetAuthToken retrieves the authentication token for a server from the configured environment variable.
func (c *ServerConfig) GetAuthToken() string {
	if c.TokenEnv == "" {
		return ""
	}
	return os.Getenv(c.TokenEnv)
}

// Validate validates the server configuration.
func (c *ServerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("server name is required")
	}

	switch c.Type {
	case ServerTypeLocal:
		// Local servers don't need endpoints
	case ServerTypeRemote:
		if c.Endpoint == "" {
			return fmt.Errorf("remote server %q requires an endpoint", c.Name)
		}
	default:
		return fmt.Errorf("invalid server type for %q: %v", c.Name, c.Type)
	}

	switch c.AuthType {
	case AuthTypeOAuth2, AuthTypeAPIKey:
		if c.TokenEnv == "" {
			return fmt.Errorf("server %q with auth type %v requires a token environment variable", c.Name, c.AuthType)
		}
	case AuthTypeNone:
		// No auth required
	default:
		return fmt.Errorf("invalid auth type for server %q: %v", c.Name, c.AuthType)
	}

	return nil
}
