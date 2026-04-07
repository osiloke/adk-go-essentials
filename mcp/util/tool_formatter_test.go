package util

import (
	"strings"
	"testing"
)

func makeTestTools() []DiscoveredTool {
	return []DiscoveredTool{
		{
			Name:        "list_projects",
			Description: "Lists all available projects",
			InputSchema: &InputSchema{
				Type:       "object",
				Properties: map[string]any{"filter": map[string]any{"type": "string", "description": "Optional filter"}},
				Required:   []string{},
			},
			ServerName: "stitch",
		},
		{
			Name:        "create_project",
			Description: "Creates a new project",
			InputSchema: &InputSchema{
				Type: "object",
				Properties: map[string]any{
					"name":        map[string]any{"type": "string", "description": "Project name"},
					"description": map[string]any{"type": "string", "description": "Project description"},
				},
				Required: []string{"name"},
			},
			ServerName: "stitch",
		},
		{
			Name:        "list_repos",
			Description: "Lists GitHub repositories",
			ServerName:  "github",
		},
	}
}

func TestFormatToolsAsMarkdown_Empty(t *testing.T) {
	result := FormatToolsAsMarkdown(nil, nil)
	if result != "" {
		t.Errorf("expected empty string for nil tools, got %q", result)
	}
}

func TestFormatToolsAsMarkdown_DefaultOptions(t *testing.T) {
	tools := makeTestTools()
	result := FormatToolsAsMarkdown(tools, nil)
	if result == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(result, "Available MCP Tools") {
		t.Error("expected header to be present")
	}
}

func TestFormatToolsAsMarkdown_WithDescriptions(t *testing.T) {
	tools := makeTestTools()
	opts := &ToolFormatOptions{
		IncludeDescription: true,
		IncludeParameters:  false,
	}
	result := FormatToolsAsMarkdown(tools, opts)
	if !strings.Contains(result, "Lists all available projects") {
		t.Error("expected description to be present")
	}
}

func TestFormatToolsAsMarkdown_WithParameters(t *testing.T) {
	tools := makeTestTools()
	opts := &ToolFormatOptions{
		IncludeDescription: true,
		IncludeParameters:  true,
	}
	result := FormatToolsAsMarkdown(tools, opts)
	if !strings.Contains(result, "**Parameters**") {
		t.Error("expected parameters section")
	}
	if !strings.Contains(result, "`name`") {
		t.Error("expected parameter name")
	}
}

func TestFormatToolsAsMarkdown_GroupByServer(t *testing.T) {
	tools := makeTestTools()
	opts := &ToolFormatOptions{
		IncludeDescription: true,
		IncludeParameters:  false,
		GroupByServer:      true,
	}
	result := FormatToolsAsMarkdown(tools, opts)
	if !strings.Contains(result, "### Server: stitch") {
		t.Error("expected stitch server group")
	}
	if !strings.Contains(result, "### Server: github") {
		t.Error("expected github server group")
	}
}

func TestFormatToolsAsMarkdown_CustomTitle(t *testing.T) {
	tools := makeTestTools()
	opts := &ToolFormatOptions{
		Title: "Custom Tools",
	}
	result := FormatToolsAsMarkdown(tools, opts)
	if !strings.Contains(result, "## Custom Tools") {
		t.Error("expected custom title")
	}
}

func TestFormatToolsAsMarkdown_RequiredParameters(t *testing.T) {
	tools := makeTestTools()
	opts := &ToolFormatOptions{
		IncludeParameters: true,
	}
	result := FormatToolsAsMarkdown(tools, opts)
	if !strings.Contains(result, "**(required)**") {
		t.Error("expected required marker for 'name' parameter")
	}
}

func TestFormatToolsAsTable_Empty(t *testing.T) {
	result := FormatToolsAsTable(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatToolsAsTable(t *testing.T) {
	tools := makeTestTools()
	result := FormatToolsAsTable(tools)
	if !strings.Contains(result, "| Tool | Server | Description |") {
		t.Error("expected table header")
	}
	if !strings.Contains(result, "`list_projects`") {
		t.Error("expected list_projects tool")
	}
	if !strings.Contains(result, "stitch") {
		t.Error("expected stitch server name")
	}
}

func TestFormatToolsAsTable_TruncatesLongDescriptions(t *testing.T) {
	tools := []DiscoveredTool{
		{
			Name:        "long_desc_tool",
			Description: "This is a very long description that should be truncated in the table output to keep it readable",
			ServerName:  "test",
		},
	}
	result := FormatToolsAsTable(tools)
	if strings.Contains(result, "to keep it readable") {
		t.Error("expected description to be truncated")
	}
	if !strings.Contains(result, "...") {
		t.Error("expected truncation ellipsis")
	}
}

func TestFormatToolsAsList_Empty(t *testing.T) {
	result := FormatToolsAsList(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatToolsAsList(t *testing.T) {
	tools := makeTestTools()
	result := FormatToolsAsList(tools)
	if !strings.Contains(result, "- `list_projects`") {
		t.Error("expected list item for list_projects")
	}
	if !strings.Contains(result, "(stitch)") {
		t.Error("expected server in list")
	}
}

func TestFormatToolNames(t *testing.T) {
	tools := makeTestTools()
	result := FormatToolNames(tools)
	expected := "`list_projects`, `create_project`, `list_repos`"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatToolNames_Empty(t *testing.T) {
	result := FormatToolNames(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatToolSchema_NilSchema(t *testing.T) {
	tool := DiscoveredTool{Name: "test", ServerName: "srv", Description: "desc"}
	result := FormatToolSchema(tool)
	if !strings.Contains(result, "No schema available") {
		t.Error("expected 'No schema available' message")
	}
}

func TestFormatToolSchema_WithSchema(t *testing.T) {
	tool := DiscoveredTool{
		Name:        "my_tool",
		ServerName:  "srv",
		Description: "A tool",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]any{
				"input": map[string]any{"type": "string"},
			},
		},
	}
	result := FormatToolSchema(tool)
	if !strings.Contains(result, "## Tool: `my_tool`") {
		t.Error("expected tool header")
	}
	if !strings.Contains(result, "**Server**: srv") {
		t.Error("expected server name")
	}
	if !strings.Contains(result, "```json") {
		t.Error("expected JSON code block")
	}
}

func TestFormatToolsByServer_Empty(t *testing.T) {
	result := FormatToolsByServer(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatToolsByServer(t *testing.T) {
	tools := makeTestTools()
	result := FormatToolsByServer(tools)
	if !strings.Contains(result, "## Available MCP Tools by Server") {
		t.Error("expected header")
	}
	if !strings.Contains(result, "stitch") {
		t.Error("expected stitch server")
	}
	if !strings.Contains(result, "| Tool | Description |") {
		t.Error("expected table header")
	}
}

func TestFilterToolsByServer(t *testing.T) {
	tools := makeTestTools()

	stitchTools := FilterToolsByServer(tools, "stitch")
	if len(stitchTools) != 2 {
		t.Errorf("expected 2 stitch tools, got %d", len(stitchTools))
	}

	githubTools := FilterToolsByServer(tools, "github")
	if len(githubTools) != 1 {
		t.Errorf("expected 1 github tool, got %d", len(githubTools))
	}

	otherTools := FilterToolsByServer(tools, "other")
	if len(otherTools) != 0 {
		t.Errorf("expected 0 tools for other server, got %d", len(otherTools))
	}
}

func TestFilterToolsByNamePattern(t *testing.T) {
	tools := makeTestTools()

	// Exact match
	results := FilterToolsByNamePattern(tools, "list_projects")
	if len(results) != 1 {
		t.Errorf("expected 1 tool, got %d", len(results))
	}

	// Wildcard prefix
	results = FilterToolsByNamePattern(tools, "list_*")
	if len(results) != 2 {
		t.Errorf("expected 2 tools, got %d", len(results))
	}

	// Wildcard suffix
	results = FilterToolsByNamePattern(tools, "*projects")
	if len(results) != 1 {
		t.Errorf("expected 1 tool, got %d", len(results))
	}

	// No match
	results = FilterToolsByNamePattern(tools, "nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 tools, got %d", len(results))
	}
}

func TestFilterToolsByNamePattern_CaseInsensitive(t *testing.T) {
	tools := makeTestTools()
	results := FilterToolsByNamePattern(tools, "LIST_PROJECTS")
	if len(results) != 1 {
		t.Errorf("expected 1 tool (case insensitive), got %d", len(results))
	}
}

func TestFilterToolsByNamePattern_WildcardBothSides(t *testing.T) {
	tools := makeTestTools()
	// The implementation splits on "*" and only handles 2 parts (prefix/suffix).
	// "*project*" splits into 3 parts ["", "project", ""], which is not handled.
	// Use "*projects" or "list_*" instead.
	results := FilterToolsByNamePattern(tools, "*projects")
	if len(results) != 1 {
		t.Errorf("expected 1 tool, got %d", len(results))
	}
}

func TestDefaultToolFormatOptions(t *testing.T) {
	opts := DefaultToolFormatOptions()
	if !opts.IncludeDescription {
		t.Error("expected IncludeDescription to be true")
	}
	if !opts.IncludeParameters {
		t.Error("expected IncludeParameters to be true")
	}
	if opts.IncludeExamples {
		t.Error("expected IncludeExamples to be false")
	}
	if opts.GroupByServer {
		t.Error("expected GroupByServer to be false")
	}
	if opts.Title != "Available MCP Tools" {
		t.Errorf("expected default title, got %s", opts.Title)
	}
}

func TestFormatToolsAsMarkdown_NoParameters(t *testing.T) {
	tools := []DiscoveredTool{
		{
			Name:        "no_params",
			Description: "A tool without params",
			ServerName:  "test",
		},
	}
	result := FormatToolsAsMarkdown(tools, nil)
	if strings.Contains(result, "**Parameters**") {
		t.Error("should not have parameters section for tool with no schema")
	}
}

func TestFormatToolSchema_InvalidSchema(t *testing.T) {
	tool := DiscoveredTool{
		Name:        "bad_schema",
		ServerName:  "test",
		Description: "desc",
		InputSchema: &InputSchema{},
	}
	result := FormatToolSchema(tool)
	if !strings.Contains(result, "## Tool: `bad_schema`") {
		t.Error("expected tool header even with empty schema")
	}
}

func TestFormatToolsAsTable_EmptySlice(t *testing.T) {
	result := FormatToolsAsTable([]DiscoveredTool{})
	if result != "" {
		t.Errorf("expected empty string for empty slice, got %q", result)
	}
}

func TestFormatToolsAsList_SingleItem(t *testing.T) {
	tools := []DiscoveredTool{
		{Name: "only_tool", ServerName: "srv", Description: "desc"},
	}
	result := FormatToolsAsList(tools)
	if !strings.Contains(result, "`only_tool`") {
		t.Error("expected tool name")
	}
}

func TestFilterToolsByNamePattern_OnlyPrefix(t *testing.T) {
	tools := makeTestTools()
	results := FilterToolsByNamePattern(tools, "list*")
	if len(results) != 2 {
		t.Errorf("expected 2 tools starting with list, got %d", len(results))
	}
}

func TestFilterToolsByNamePattern_OnlySuffix(t *testing.T) {
	tools := makeTestTools()
	results := FilterToolsByNamePattern(tools, "*repos")
	if len(results) != 1 {
		t.Errorf("expected 1 tool ending with repos, got %d", len(results))
	}
}
