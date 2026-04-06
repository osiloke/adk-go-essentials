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

package util

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// ContextValue represents a value stored in the context manager.
type ContextValue struct {
	// Key is the context key.
	Key string `json:"key"`
	// Value is the stored value.
	Value any `json:"value"`
	// UpdatedAt is when the value was last updated.
	UpdatedAt string `json:"updated_at"`
	// Source indicates what set this value (e.g., tool name).
	Source string `json:"source,omitempty"`
}

// ContextManager manages contextual state for MCP tool calls.
// It is safe for concurrent use.
//
// This is a generic framework that can be specialized for specific use cases
// like Stitch project context, GitHub repo context, etc.
type ContextManager struct {
	mu    sync.RWMutex
	state map[string]*ContextValue
}

// NewContextManager creates a new context manager.
func NewContextManager() *ContextManager {
	return &ContextManager{
		state: make(map[string]*ContextValue),
	}
}

// Set stores a value in the context.
func (m *ContextManager) Set(key string, value any, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state[key] = &ContextValue{
		Key:       key,
		Value:     value,
		UpdatedAt: currentTime(),
		Source:    source,
	}
	return nil
}

// Get retrieves a value from the context.
// Returns (value, true) if found, (nil, false) if not found.
func (m *ContextManager) Get(key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.state[key]
	if !ok {
		return nil, false
	}
	return val.Value, true
}

// GetString retrieves a string value from the context.
// Returns (value, true) if found and is string, ("", false) otherwise.
func (m *ContextManager) GetString(key string) (string, bool) {
	val, ok := m.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt retrieves an int value from the context.
// Returns (value, true) if found and is int, (0, false) otherwise.
func (m *ContextManager) GetInt(key string) (int, bool) {
	val, ok := m.Get(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

// GetMap retrieves a map value from the context.
// Returns (value, true) if found and is map, (nil, false) otherwise.
func (m *ContextManager) GetMap(key string) (map[string]any, bool) {
	val, ok := m.Get(key)
	if !ok {
		return nil, false
	}
	mp, ok := val.(map[string]any)
	return mp, ok
}

// GetAll returns all context values.
func (m *ContextManager) GetAll() map[string]*ContextValue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ContextValue)
	for k, v := range m.state {
		result[k] = v
	}
	return result
}

// GetKeys returns all context keys.
func (m *ContextManager) GetKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.state))
	for k := range m.state {
		keys = append(keys, k)
	}
	return keys
}

// Has checks if a key exists in the context.
func (m *ContextManager) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.state[key]
	return ok
}

// Clear removes all context values.
func (m *ContextManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = make(map[string]*ContextValue)
}

// Delete removes a specific key from the context.
func (m *ContextManager) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.state, key)
}

// DeletePrefix removes all keys with a given prefix.
func (m *ContextManager) DeletePrefix(prefix string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for k := range m.state {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.state, k)
			count++
		}
	}
	return count
}

// ToJSON exports the context as JSON.
func (m *ContextManager) ToJSON() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON imports context from JSON.
func (m *ContextManager) FromJSON(jsonStr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var state map[string]*ContextValue
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		return err
	}
	m.state = state
	return nil
}

// Size returns the number of items in the context.
func (m *ContextManager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.state)
}

// NewSetContextTool creates a generic tool for setting context values.
func NewSetContextTool(manager *ContextManager) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "set_context",
		Description: "Sets a value in the MCP context manager. Use this to persist important values across tool calls.",
	}, func(ctx tool.Context, args struct {
		Key    string `json:"key" description:"The context key to set"`
		Value  string `json:"value" description:"The value to store (as JSON string)"`
		Source string `json:"source,omitempty" description:"Optional source identifier"`
	}) (string, error) {
		// Parse value as JSON if possible
		var parsedValue any
		if err := json.Unmarshal([]byte(args.Value), &parsedValue); err != nil {
			// If not valid JSON, use as string
			parsedValue = args.Value
		}

		if err := manager.Set(args.Key, parsedValue, args.Source); err != nil {
			return "", fmt.Errorf("failed to set context: %w", err)
		}

		return fmt.Sprintf("Context key %q set successfully", args.Key), nil
	})
}

// NewGetContextTool creates a generic tool for getting context values.
func NewGetContextTool(manager *ContextManager) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "get_context",
		Description: "Retrieves a value from the MCP context manager.",
	}, func(ctx tool.Context, args struct {
		Key string `json:"key" description:"The context key to retrieve"`
	}) (string, error) {
		val, ok := manager.Get(args.Key)
		if !ok {
			return fmt.Sprintf("Context key %q not found", args.Key), nil
		}

		// Format as JSON for structured values
		if data, err := json.MarshalIndent(val, "", "  "); err == nil {
			return string(data), nil
		}

		return fmt.Sprintf("%v", val), nil
	})
}

// NewListContextTool creates a generic tool for listing all context keys.
func NewListContextTool(manager *ContextManager) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "list_context",
		Description: "Lists all keys currently stored in the MCP context manager.",
	}, func(ctx tool.Context, _ struct{}) (string, error) {
		keys := manager.GetKeys()
		if len(keys) == 0 {
			return "Context is empty", nil
		}

		var sb strings.Builder
		sb.WriteString("Context keys:\n")
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("- %s\n", k))
		}
		return sb.String(), nil
	})
}

// NewClearContextTool creates a generic tool for clearing the context.
func NewClearContextTool(manager *ContextManager) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "clear_context",
		Description: "Clears all values from the MCP context manager. Use with caution.",
	}, func(ctx tool.Context, args struct {
		Confirm bool `json:"confirm" description:"Set to true to confirm clearing all context"`
	}) (string, error) {
		if !args.Confirm {
			return "Confirmation required. Set confirm=true to clear all context.", nil
		}

		manager.Clear()
		return "Context cleared successfully", nil
	})
}

func currentTime() string {
	return time.Now().Format(time.RFC3339)
}
