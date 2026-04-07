package util

import (
	"testing"
)

func TestComposeCallbacks_NilCallbacks(t *testing.T) {
	composed := ComposeCallbacks()
	if composed == nil {
		t.Fatal("ComposeCallbacks should not return nil")
	}
}

func TestConditionalCallback_Nil(t *testing.T) {
	cb := ConditionalCallback(nil, nil)
	if cb == nil {
		t.Fatal("ConditionalCallback should not return nil")
	}
}

func TestIfHasKey_Nil(t *testing.T) {
	cb := IfHasKey("key", nil)
	if cb == nil {
		t.Fatal("IfHasKey should not return nil")
	}
}

func TestToolFormatOptions_ZeroValue(t *testing.T) {
	opts := ToolFormatOptions{}
	if opts.IncludeDescription {
		t.Error("expected IncludeDescription false by default")
	}
}

func TestDefaultToolFormatOptions_NotNil(t *testing.T) {
	opts := DefaultToolFormatOptions()
	if opts == nil {
		t.Fatal("DefaultToolFormatOptions should not return nil")
	}
}

func TestFormatToolsAsMarkdown_NilOptions(t *testing.T) {
	tools := []DiscoveredTool{
		{Name: "test", Description: "A test", ServerName: "srv"},
	}
	// Should not panic with nil options
	result := FormatToolsAsMarkdown(tools, nil)
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestFormatToolsAsTable_NilInput(t *testing.T) {
	result := FormatToolsAsTable(nil)
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

func TestFormatToolsAsList_NilInput(t *testing.T) {
	result := FormatToolsAsList(nil)
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

func TestFormatToolNames_NilInput(t *testing.T) {
	result := FormatToolNames(nil)
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

func TestFormatToolSchema_EmptyTool(t *testing.T) {
	tool := DiscoveredTool{}
	result := FormatToolSchema(tool)
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestFormatToolsByServer_NilInput(t *testing.T) {
	result := FormatToolsByServer(nil)
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

func TestFilterToolsByServer_EmptyList(t *testing.T) {
	result := FilterToolsByServer(nil, "srv")
	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

func TestFilterToolsByNamePattern_EmptyList(t *testing.T) {
	result := FilterToolsByNamePattern(nil, "*")
	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

func TestFilterToolsByNamePattern_EmptyPattern(t *testing.T) {
	tools := []DiscoveredTool{{Name: "test", ServerName: "srv"}}
	result := FilterToolsByNamePattern(tools, "")
	// Empty pattern should not match anything
	if len(result) != 0 {
		t.Errorf("expected 0 tools for empty pattern, got %d", len(result))
	}
}

func TestNewContextManager_ConcurrentAccess(t *testing.T) {
	cm := NewContextManager()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(n int) {
			cm.Set("key", n, "test")
			cm.Get("key")
			cm.Has("key")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	// If we get here without deadlock, concurrent access works
}

func TestContextManager_DeletePrefix_EmptyPrefix(t *testing.T) {
	cm := NewContextManager()
	cm.Set("a", 1, "test")
	cm.Set("b", 2, "test")

	count := cm.DeletePrefix("")
	if count != 2 {
		t.Errorf("expected to delete 2 keys with empty prefix, got %d", count)
	}
	if cm.Size() != 0 {
		t.Errorf("expected size 0, got %d", cm.Size())
	}
}

func TestContextManager_DeletePrefix_PartialMatch(t *testing.T) {
	cm := NewContextManager()
	cm.Set("abc", 1, "test")
	cm.Set("abd", 2, "test")
	cm.Set("xyz", 3, "test")

	count := cm.DeletePrefix("ab")
	if count != 2 {
		t.Errorf("expected to delete 2 keys, got %d", count)
	}
	if !cm.Has("xyz") {
		t.Error("expected xyz to still exist")
	}
}

func TestFormatToolsAsMarkdown_EmptyToolsSlice(t *testing.T) {
	result := FormatToolsAsMarkdown([]DiscoveredTool{}, nil)
	if result != "" {
		t.Errorf("expected empty string for empty tools slice, got %q", result)
	}
}

func TestIsRequired(t *testing.T) {
	required := []string{"a", "b", "c"}
	if !isRequired(required, "a") {
		t.Error("expected a to be required")
	}
	if isRequired(required, "d") {
		t.Error("expected d to not be required")
	}
	if isRequired(nil, "a") {
		t.Error("expected nil required to return false")
	}
}

func TestFormatToolsAsTable_SingleItem_UtilTest(t *testing.T) {
	tools := []DiscoveredTool{
		{Name: "tool", ServerName: "srv", Description: "desc"},
	}
	result := FormatToolsAsTable(tools)
	if !stringsContains(result, "| `tool` | srv | desc |") {
		t.Errorf("expected tool row in table, got %q", result)
	}
}

func TestFormatToolsAsList_SingleItem_UtilTest(t *testing.T) {
	tools := []DiscoveredTool{
		{Name: "tool", ServerName: "srv", Description: "desc"},
	}
	result := FormatToolsAsList(tools)
	if !stringsContains(result, "- `tool` (srv): desc") {
		t.Errorf("expected tool in list, got %q", result)
	}
}

func TestFormatToolNames_SingleItem_UtilTest(t *testing.T) {
	tools := []DiscoveredTool{{Name: "tool"}}
	result := FormatToolNames(tools)
	if result != "`tool`" {
		t.Errorf("expected `tool`, got %q", result)
	}
}

func TestFormatToolsByServer_SingleServer_UtilTest(t *testing.T) {
	tools := []DiscoveredTool{
		{Name: "t1", ServerName: "srv", Description: "desc1"},
		{Name: "t2", ServerName: "srv", Description: "desc2"},
	}
	result := FormatToolsByServer(tools)
	if !stringsContains(result, "### srv (2 tools)") {
		t.Errorf("expected server header with count, got %q", result)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (len(s) >= len(substr)) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
