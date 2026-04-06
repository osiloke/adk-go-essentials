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
)

// ToolFormatOptions configures how tools are formatted.
type ToolFormatOptions struct {
	// IncludeDescription includes tool descriptions if true.
	IncludeDescription bool
	// IncludeParameters includes parameter documentation if true.
	IncludeParameters bool
	// IncludeExamples includes usage examples if true (not yet implemented).
	IncludeExamples bool
	// GroupByServer groups tools by server if true.
	GroupByServer bool
	// Title is the main heading for the documentation.
	Title string
}

// DefaultToolFormatOptions returns sensible default formatting options.
func DefaultToolFormatOptions() *ToolFormatOptions {
	return &ToolFormatOptions{
		IncludeDescription: true,
		IncludeParameters:  true,
		IncludeExamples:    false,
		GroupByServer:      false,
		Title:              "Available MCP Tools",
	}
}

// FormatToolsAsMarkdown formats discovered tools as markdown documentation.
func FormatToolsAsMarkdown(tools []DiscoveredTool, opts *ToolFormatOptions) string {
	if opts == nil {
		opts = DefaultToolFormatOptions()
	}

	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder

	// Write header
	if opts.Title != "" {
		sb.WriteString(fmt.Sprintf("## %s\n\n", opts.Title))
	} else {
		sb.WriteString("## Available MCP Tools\n\n")
	}

	// Group by server if requested
	if opts.GroupByServer {
		serverTools := make(map[string][]DiscoveredTool)
		for _, t := range tools {
			serverTools[t.ServerName] = append(serverTools[t.ServerName], t)
		}

		for serverName, serverToolList := range serverTools {
			sb.WriteString(fmt.Sprintf("### Server: %s\n\n", serverName))
			for _, t := range serverToolList {
				writeToolDoc(&sb, t, opts)
			}
		}
	} else {
		// Flat list
		for _, t := range tools {
			writeToolDoc(&sb, t, opts)
		}
	}

	return sb.String()
}

// writeToolDoc writes documentation for a single tool.
func writeToolDoc(sb *strings.Builder, tool DiscoveredTool, opts *ToolFormatOptions) {
	sb.WriteString(fmt.Sprintf("#### `%s`\n", tool.Name))

	if opts.IncludeDescription && tool.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", tool.Description))
	}

	if opts.IncludeParameters && tool.InputSchema != nil {
		if props := tool.InputSchema.Properties; len(props) > 0 {
			sb.WriteString("**Parameters**:\n")

			for paramName, paramDef := range props {
				param, ok := paramDef.(map[string]any)
				if !ok {
					continue
				}

				paramType := param["type"]
				paramDesc := param["description"]
				required := isRequired(tool.InputSchema.Required, paramName)

				requiredStr := ""
				if required {
					requiredStr = " **(required)**"
				}

				sb.WriteString(fmt.Sprintf("- `%s` (%v)%s: %v\n",
					paramName, paramType, requiredStr, paramDesc))
			}
			sb.WriteString("\n")
		}
	}
}

// isRequired checks if a parameter is in the required list.
func isRequired(required []string, paramName string) bool {
	for _, r := range required {
		if r == paramName {
			return true
		}
	}
	return false
}

// FormatToolsAsTable formats tools as a compact markdown table.
func FormatToolsAsTable(tools []DiscoveredTool) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("| Tool | Server | Description |\n")
	sb.WriteString("|------|--------|-------------|\n")

	for _, t := range tools {
		// Truncate description for table
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", t.Name, t.ServerName, desc))
	}

	return sb.String()
}

// FormatToolsAsList formats tools as a simple bullet list.
func FormatToolsAsList(tools []DiscoveredTool) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", t.Name, t.ServerName, t.Description))
	}
	return sb.String()
}

// FormatToolNames returns a comma-separated list of tool names with backticks.
func FormatToolNames(tools []DiscoveredTool) string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = fmt.Sprintf("`%s`", t.Name)
	}
	return strings.Join(names, ", ")
}

// FormatToolSchema returns a formatted JSON schema for a tool.
func FormatToolSchema(tool DiscoveredTool) string {
	if tool.InputSchema == nil {
		return fmt.Sprintf("No schema available for tool `%s`", tool.Name)
	}

	data, err := json.MarshalIndent(tool.InputSchema, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting schema: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Tool: `%s`\n\n", tool.Name))
	sb.WriteString(fmt.Sprintf("**Server**: %s\n\n", tool.ServerName))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", tool.Description))
	sb.WriteString("**Schema**:\n```json\n")
	sb.WriteString(string(data))
	sb.WriteString("\n```\n")

	return sb.String()
}

// FormatToolsByServer formats tools grouped by server with summaries.
func FormatToolsByServer(tools []DiscoveredTool) string {
	if len(tools) == 0 {
		return ""
	}

	// Group tools by server
	serverTools := make(map[string][]DiscoveredTool)
	for _, t := range tools {
		serverTools[t.ServerName] = append(serverTools[t.ServerName], t)
	}

	var sb strings.Builder
	sb.WriteString("## Available MCP Tools by Server\n\n")

	for serverName, serverToolList := range serverTools {
		sb.WriteString(fmt.Sprintf("### %s (%d tools)\n\n", serverName, len(serverToolList)))
		sb.WriteString("| Tool | Description |\n")
		sb.WriteString("|------|-------------|\n")

		for _, t := range serverToolList {
			desc := t.Description
			if len(desc) > 70 {
				desc = desc[:67] + "..."
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", t.Name, desc))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FilterToolsByServer returns only tools from a specific server.
func FilterToolsByServer(tools []DiscoveredTool, serverName string) []DiscoveredTool {
	var result []DiscoveredTool
	for _, t := range tools {
		if t.ServerName == serverName {
			result = append(result, t)
		}
	}
	return result
}

// FilterToolsByNamePattern returns tools matching a name pattern.
// Pattern supports simple wildcard matching with * (e.g., "stitch_*").
func FilterToolsByNamePattern(tools []DiscoveredTool, pattern string) []DiscoveredTool {
	var result []DiscoveredTool

	// Convert pattern to lowercase for case-insensitive matching
	pattern = strings.ToLower(pattern)

	for _, t := range tools {
		name := strings.ToLower(t.Name)

		// Handle wildcard pattern
		if strings.Contains(pattern, "*") {
			parts := strings.Split(pattern, "*")
			if len(parts) == 2 {
				prefix := parts[0]
				suffix := parts[1]

				if (prefix == "" || strings.HasPrefix(name, prefix)) &&
					(suffix == "" || strings.HasSuffix(name, suffix)) {
					result = append(result, t)
				}
			}
		} else if name == pattern {
			result = append(result, t)
		}
	}

	return result
}
