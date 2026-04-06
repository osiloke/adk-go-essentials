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
)

// Integration provides helper functions for integrating MCP tools with ADK agents.
type Integration struct {
	registry *Registry
}

// NewIntegration creates a new MCP integration with the given registry.
func NewIntegration(registry *Registry) *Integration {
	return &Integration{registry: registry}
}

// InitIntegration initializes MCP integration from environment configuration.
// This is a convenience function that combines InitMCPRegistry and NewIntegration.
func InitIntegration(ctx context.Context) (*Integration, error) {
	observability.Log.Debugf("[MCP Integration] Initializing MCP integration from environment")

	registry, err := InitMCPRegistry(ctx)
	if err != nil {
		observability.Log.Errorf("[MCP Integration] Failed to initialize MCP registry: %v", err)
		return nil, err
	}

	observability.Log.Debugf("[MCP Integration] MCP integration initialized successfully")
	return NewIntegration(registry), nil
}

// GetToolset returns the toolset for a specific MCP server.
func (i *Integration) GetToolset(serverName string) (tool.Toolset, error) {
	return i.registry.GetToolset(serverName)
}

// GetDefaultToolset returns the default MCP toolset.
func (i *Integration) GetDefaultToolset() (tool.Toolset, error) {
	return i.registry.GetDefaultToolset()
}

// GetAllToolsets returns all available MCP toolsets.
func (i *Integration) GetAllToolsets() ([]tool.Toolset, error) {
	return i.registry.GetAllToolsets()
}

// GetToolsetByName is a convenience function that gets a toolset by server name.
// It can be called directly without creating an Integration instance.
func GetToolsetByName(serverName string) (tool.Toolset, error) {
	// This requires the registry to be initialized globally
	// For now, users should use InitIntegration and call methods on the returned instance
	return nil, fmt.Errorf("use InitIntegration() and call GetToolset() on the returned instance")
}

// Close shuts down all MCP connections.
func (i *Integration) Close() error {
	return i.registry.Close()
}

// Registry returns the underlying MCP registry.
func (i *Integration) Registry() *Registry {
	return i.registry
}
