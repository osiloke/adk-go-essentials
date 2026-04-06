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

	"github.com/osiloke/adk-go-essentials/observability"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// PromptInjector provides utilities for injecting MCP-related information
// into LLM prompts. It is safe for concurrent use.
type PromptInjector struct {
	discoveryService *ToolDiscoveryService
	contextManager   *ContextManager
}

// NewPromptInjector creates a new prompt injector.
// Either discoveryService or contextManager can be nil if not needed.
func NewPromptInjector(
	discoveryService *ToolDiscoveryService,
	contextManager *ContextManager,
) *PromptInjector {
	return &PromptInjector{
		discoveryService: discoveryService,
		contextManager:   contextManager,
	}
}

// InjectToolsCallback creates a BeforeModelCallback that injects tool documentation
// into the LLM prompt. The tools are retrieved from the discovery service cache.
func (i *PromptInjector) InjectToolsCallback(serverName string, opts *ToolFormatOptions) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		observability.Log.Infof("[PromptInjector] InjectToolsCallback START for server %q", serverName)

		if i.discoveryService == nil {
			observability.Log.Warnf("[PromptInjector] discoveryService is nil, returning")
			return nil, nil
		}

		// Get tools from cache
		tools, ok := i.discoveryService.GetTools(serverName)
		observability.Log.Infof("[PromptInjector] GetTools(%q) returned ok=%v, len=%d", serverName, ok, len(tools))
		if !ok || len(tools) == 0 {
			observability.Log.Warnf("[PromptInjector] No tools found for server %q, returning without injection", serverName)
			return nil, nil
		}

		// Format tools as markdown
		if opts == nil {
			opts = DefaultToolFormatOptions()
		}

		toolDocs := FormatToolsAsMarkdown(tools, opts)
		if toolDocs == "" {
			observability.Log.Warnf("[PromptInjector] FormatToolsAsMarkdown returned empty string")
			return nil, nil
		}

		// Log tool injection
		var toolNames []string
		for _, t := range tools {
			toolNames = append(toolNames, t.Name)
		}
		observability.Log.Infof("[PromptInjector] Injecting %d MCP tools from %q into prompt: [%s]", len(tools), serverName, strings.Join(toolNames, ", "))

		// Inject into system instruction
		injectText(req.Config.SystemInstruction, toolDocs)
		observability.Log.Infof("[PromptInjector] InjectToolsCallback END - tools injected successfully")
		return nil, nil
	}
}

// InjectAllToolsCallback creates a BeforeModelCallback that injects documentation
// for all discovered tools from all servers.
func (i *PromptInjector) InjectAllToolsCallback(opts *ToolFormatOptions) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		if i.discoveryService == nil {
			return nil, nil
		}

		allTools := i.discoveryService.GetAllTools()
		if len(allTools) == 0 {
			return nil, nil
		}

		if opts == nil {
			opts = &ToolFormatOptions{
				IncludeDescription: true,
				IncludeParameters:  true,
				GroupByServer:      true,
				Title:              "Available MCP Tools",
			}
		}

		toolDocs := FormatToolsAsMarkdown(allTools, opts)
		if toolDocs == "" {
			return nil, nil
		}

		injectText(req.Config.SystemInstruction, toolDocs)
		return nil, nil
	}
}

// InjectContextCallback creates a BeforeModelCallback that injects context values
// into the LLM prompt.
func (i *PromptInjector) InjectContextCallback(keys ...string) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		if i.contextManager == nil {
			return nil, nil
		}

		var contextLines []string
		var injectedKeys []string

		for _, key := range keys {
			val, ok := i.contextManager.Get(key)
			if !ok {
				continue
			}

			injectedKeys = append(injectedKeys, key)
			// Format value
			formatted := formatContextValue(val)
			contextLines = append(contextLines, fmt.Sprintf("- **%s**: %s", key, formatted))
		}

		if len(contextLines) == 0 {
			return nil, nil
		}

		observability.Log.Infof("[PromptInjector] Injecting context keys into prompt: [%s]", strings.Join(injectedKeys, ", "))

		injection := fmt.Sprintf(`
### MCP Context
The following context values are available:
%s

Use these values when making MCP tool calls that require them.
`, strings.Join(contextLines, "\n"))

		injectText(req.Config.SystemInstruction, injection)
		return nil, nil
	}
}

// InjectAllContextCallback creates a BeforeModelCallback that injects all context
// values into the LLM prompt.
func (i *PromptInjector) InjectAllContextCallback() llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		if i.contextManager == nil {
			return nil, nil
		}

		allContext := i.contextManager.GetAll()
		if len(allContext) == 0 {
			return nil, nil
		}

		var contextLines []string
		for key, cv := range allContext {
			formatted := formatContextValue(cv.Value)
			contextLines = append(contextLines, fmt.Sprintf("- **%s**: %s", key, formatted))
		}

		injection := fmt.Sprintf(`
### MCP Context
The following context values are available:
%s
`, strings.Join(contextLines, "\n"))

		injectText(req.Config.SystemInstruction, injection)
		return nil, nil
	}
}

// InjectStateCallback creates a BeforeModelCallback that injects values from
// session state into the LLM prompt.
func InjectStateCallback(keys ...string) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		state := ctx.State()
		if state == nil {
			return nil, nil
		}

		var contextLines []string
		var injectedKeys []string

		for _, key := range keys {
			val, err := state.Get(key)
			if err != nil || val == nil {
				continue
			}

			injectedKeys = append(injectedKeys, key)
			formatted := formatContextValue(val)
			contextLines = append(contextLines, fmt.Sprintf("- **%s**: %s", key, formatted))
		}

		if len(contextLines) == 0 {
			return nil, nil
		}

		observability.Log.Infof("[PromptInjector] Injecting state keys into prompt: [%s]", strings.Join(injectedKeys, ", "))

		injection := fmt.Sprintf(`
### Session State Context
The following values are available from session state:
%s
`, strings.Join(contextLines, "\n"))

		injectText(req.Config.SystemInstruction, injection)
		return nil, nil
	}
}

// InjectStateAsSection creates a BeforeModelCallback that injects session state
// values with a custom section title.
func InjectStateAsSection(sectionTitle string, keys ...string) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		state := ctx.State()
		if state == nil {
			return nil, nil
		}

		var contextLines []string

		for _, key := range keys {
			val, err := state.Get(key)
			if err != nil || val == nil {
				continue
			}

			formatted := formatContextValue(val)
			contextLines = append(contextLines, fmt.Sprintf("- **%s**: %s", key, formatted))
		}

		if len(contextLines) == 0 {
			return nil, nil
		}

		injection := fmt.Sprintf(`
### %s
%s
`, sectionTitle, strings.Join(contextLines, "\n"))

		injectText(req.Config.SystemInstruction, injection)
		return nil, nil
	}
}

// formatContextValue formats a value for display in the prompt.
func formatContextValue(val any) string {
	// Try to format as JSON for structured values
	if data, err := json.MarshalIndent(val, "", "  "); err == nil {
		// For simple values, keep on one line
		if s, ok := val.(string); ok {
			return s
		}
		if _, ok := val.(map[string]any); ok {
			return string(data)
		}
		if _, ok := val.([]any); ok {
			return string(data)
		}
		return string(data)
	}
	return fmt.Sprintf("%v", val)
}

// injectText adds text to the system instruction.
func injectText(instruction *genai.Content, text string) {
	if instruction == nil {
		// Create new system instruction
		// Note: This won't persist since we're modifying a copy
		// The caller should handle this case
		return
	}

	if len(instruction.Parts) > 0 {
		instruction.Parts[0].Text += text
	} else {
		instruction.Parts = append(instruction.Parts, &genai.Part{Text: text})
	}
}

// ComposeCallbacks combines multiple callbacks into one.
// All callbacks are executed in order.
func ComposeCallbacks(callbacks ...llmagent.BeforeModelCallback) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		var lastResp *model.LLMResponse
		var lastErr error

		for _, cb := range callbacks {
			resp, err := cb(ctx, req)
			if err != nil {
				lastErr = err
			}
			if resp != nil {
				lastResp = resp
			}
		}

		return lastResp, lastErr
	}
}

// ConditionalCallback creates a callback that only executes if a condition is met.
func ConditionalCallback(
	condition func(ctx agent.CallbackContext) bool,
	callback llmagent.BeforeModelCallback,
) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		if !condition(ctx) {
			return nil, nil
		}
		return callback(ctx, req)
	}
}

// IfHasKey creates a conditional callback that only executes if the session state
// contains the specified key.
func IfHasKey(key string, callback llmagent.BeforeModelCallback) llmagent.BeforeModelCallback {
	return ConditionalCallback(
		func(ctx agent.CallbackContext) bool {
			state := ctx.State()
			if state == nil {
				return false
			}
			val, err := state.Get(key)
			return err == nil && val != nil
		},
		callback,
	)
}
