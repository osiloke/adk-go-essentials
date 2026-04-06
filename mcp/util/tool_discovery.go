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

// Package util provides generic MCP utilities for tool discovery, context management,
// and prompt injection. These utilities work with any MCP server.
package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/osiloke/adk-go-essentials/mcp"
	"github.com/osiloke/adk-go-essentials/observability"
)

// DiscoveredTool represents a tool discovered from an MCP server.
type DiscoveredTool struct {
	// Name is the tool's unique identifier.
	Name string `json:"name"`
	// Description explains what the tool does.
	Description string `json:"description"`
	// InputSchema describes the tool's input parameters.
	InputSchema *InputSchema `json:"input_schema,omitempty"`
	// ServerName is the MCP server that provides this tool.
	ServerName string `json:"server_name"`
	// DiscoveredAt is when the tool was discovered.
	DiscoveredAt time.Time `json:"discovered_at"`
}

// InputSchema represents a tool's input parameter schema.
type InputSchema struct {
	// Type is the JSON schema type (usually "object").
	Type string `json:"type,omitempty"`
	// Properties maps parameter names to their schemas.
	Properties map[string]any `json:"properties,omitempty"`
	// Required lists required parameter names.
	Required []string `json:"required,omitempty"`
}

// ToolDiscoveryService manages MCP tool discovery and caching across all servers.
// It is safe for concurrent use.
type ToolDiscoveryService struct {
	mu          sync.RWMutex
	cache       map[string][]DiscoveredTool // server_name -> tools
	cacheExpiry map[string]time.Time
	cacheTTL    time.Duration
}

// NewToolDiscoveryService creates a new tool discovery service.
// cacheTTL specifies how long discovered tools are cached before refresh.
func NewToolDiscoveryService(cacheTTL time.Duration) *ToolDiscoveryService {
	if cacheTTL <= 0 {
		cacheTTL = 15 * time.Minute // Default TTL
	}
	return &ToolDiscoveryService{
		cache:       make(map[string][]DiscoveredTool),
		cacheExpiry: make(map[string]time.Time),
		cacheTTL:    cacheTTL,
	}
}

// DiscoverTools retrieves tools from an MCP server and caches them.
// If tools are already cached and not expired, cached results are returned.
func (s *ToolDiscoveryService) DiscoverTools(
	ctx context.Context,
	client *mcp.Client,
	serverName string,
) ([]DiscoveredTool, error) {
	// Check cache first
	s.mu.RLock()
	if tools, ok := s.cache[serverName]; ok {
		if expiry, ok := s.cacheExpiry[serverName]; ok && time.Now().Before(expiry) {
			s.mu.RUnlock()
			observability.Log.Debugf("[ToolDiscovery] Using cached tools for server %q (%d tools)", serverName, len(tools))
			return tools, nil
		}
	}
	s.mu.RUnlock()

	// Discover tools from MCP server
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from server %q: %w", serverName, err)
	}

	// Convert to our format
	tools := make([]DiscoveredTool, 0, len(mcpTools))
	for _, t := range mcpTools {
		var schema *InputSchema
		if t.InputSchema != nil {
			schema = parseInputSchema(any(t.InputSchema))
		}

		tools = append(tools, DiscoveredTool{
			Name:         t.Name,
			Description:  t.Description,
			InputSchema:  schema,
			ServerName:   serverName,
			DiscoveredAt: time.Now(),
		})
	}

	// Cache the results
	s.mu.Lock()
	s.cache[serverName] = tools
	s.cacheExpiry[serverName] = time.Now().Add(s.cacheTTL)
	s.mu.Unlock()

	// Log discovered tools in a single line
	var toolNames []string
	for _, t := range tools {
		toolNames = append(toolNames, t.Name)
	}
	observability.Log.Infof("[ToolDiscovery] Server %q: %d tools [%s]", serverName, len(tools), strings.Join(toolNames, ", "))
	return tools, nil
}

// GetTools returns cached tools for a server.
// Returns (tools, true) if found, (nil, false) if not cached.
func (s *ToolDiscoveryService) GetTools(serverName string) ([]DiscoveredTool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tools, ok := s.cache[serverName]
	return tools, ok
}

// GetAllTools returns all cached tools across all servers.
func (s *ToolDiscoveryService) GetAllTools() []DiscoveredTool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var allTools []DiscoveredTool
	for _, tools := range s.cache {
		allTools = append(allTools, tools...)
	}
	return allTools
}

// GetToolByName finds a specific tool by name across all servers.
// Returns (tool, true) if found, (nil, false) if not found.
func (s *ToolDiscoveryService) GetToolByName(toolName string) (*DiscoveredTool, bool) {
	allTools := s.GetAllTools()
	for _, t := range allTools {
		if t.Name == toolName {
			return &t, true
		}
	}
	return nil, false
}

// GetToolsByServer returns tools for a specific server.
// Returns (tools, true) if found, (nil, false) if not cached.
func (s *ToolDiscoveryService) GetToolsByServer(serverName string) ([]DiscoveredTool, bool) {
	return s.GetTools(serverName)
}

// ClearCache clears the cache for specific servers or all servers.
// If no server names are provided, all caches are cleared.
func (s *ToolDiscoveryService) ClearCache(serverNames ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(serverNames) == 0 {
		s.cache = make(map[string][]DiscoveredTool)
		s.cacheExpiry = make(map[string]time.Time)
		observability.Log.Debugf("[ToolDiscovery] Cleared all caches")
	} else {
		for _, name := range serverNames {
			delete(s.cache, name)
			delete(s.cacheExpiry, name)
		}
		observability.Log.Debugf("[ToolDiscovery] Cleared cache for servers: %v", serverNames)
	}
}

// GetCacheStats returns cache statistics.
func (s *ToolDiscoveryService) GetCacheStats() (serverCount int, totalTools int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	serverCount = len(s.cache)
	for _, tools := range s.cache {
		totalTools += len(tools)
	}
	return serverCount, totalTools
}

// parseInputSchema parses the SDK's jsonschema.Schema into our format.
func parseInputSchema(schema any) *InputSchema {
	if schema == nil {
		return nil
	}

	// Marshal to JSON first to normalize the structure
	data, err := json.Marshal(schema)
	if err != nil {
		observability.Log.Warnf("[ToolDiscovery] Failed to marshal schema: %v", err)
		return nil
	}

	// Unmarshal into our format
	var result InputSchema
	if err := json.Unmarshal(data, &result); err != nil {
		observability.Log.Warnf("[ToolDiscovery] Failed to unmarshal schema: %v", err)
		return nil
	}

	return &result
}
