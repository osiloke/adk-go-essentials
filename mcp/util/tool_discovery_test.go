package util

import (
	"testing"
	"time"
)

func TestNewToolDiscoveryService_DefaultTTL(t *testing.T) {
	ds := NewToolDiscoveryService(0)
	if ds == nil {
		t.Fatal("NewToolDiscoveryService() returned nil")
	}
	if ds.cacheTTL <= 0 {
		t.Error("cacheTTL should be positive")
	}
}

func TestNewToolDiscoveryService_CustomTTL(t *testing.T) {
	ds := NewToolDiscoveryService(5 * time.Minute)
	if ds.cacheTTL != 5*time.Minute {
		t.Errorf("expected TTL 5m, got %v", ds.cacheTTL)
	}
}

func TestNewToolDiscoveryService_NegativeTTL_UsesDefault(t *testing.T) {
	ds := NewToolDiscoveryService(-1 * time.Minute)
	if ds.cacheTTL <= 0 {
		t.Error("negative TTL should default to positive value")
	}
}

func TestToolDiscoveryService_GetTools_NotCached(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	_, ok := ds.GetTools("unknown")
	if ok {
		t.Error("expected ok=false for uncached server")
	}
}

func TestToolDiscoveryService_GetAllTools_Empty(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	tools := ds.GetAllTools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestToolDiscoveryService_GetToolByName_NotFound(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	_, ok := ds.GetToolByName("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent tool")
	}
}

func TestToolDiscoveryService_ClearCache(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	// Manually populate cache for testing
	ds.cache["srv1"] = []DiscoveredTool{{Name: "tool1", ServerName: "srv1"}}
	ds.cacheExpiry["srv1"] = time.Now().Add(time.Hour)
	ds.cache["srv2"] = []DiscoveredTool{{Name: "tool2", ServerName: "srv2"}}
	ds.cacheExpiry["srv2"] = time.Now().Add(time.Hour)

	ds.ClearCache("srv1")

	if _, ok := ds.cache["srv1"]; ok {
		t.Error("expected srv1 to be cleared")
	}
	if _, ok := ds.cache["srv2"]; !ok {
		t.Error("expected srv2 to still exist")
	}
}

func TestToolDiscoveryService_ClearCache_All(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	ds.cache["srv1"] = []DiscoveredTool{{Name: "tool1"}}
	ds.cacheExpiry["srv1"] = time.Now().Add(time.Hour)

	ds.ClearCache()

	if len(ds.cache) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(ds.cache))
	}
}

func TestToolDiscoveryService_GetCacheStats(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	ds.cache["srv1"] = []DiscoveredTool{
		{Name: "t1", ServerName: "srv1"},
		{Name: "t2", ServerName: "srv1"},
	}
	ds.cacheExpiry["srv1"] = time.Now().Add(time.Hour)
	ds.cache["srv2"] = []DiscoveredTool{{Name: "t3", ServerName: "srv2"}}
	ds.cacheExpiry["srv2"] = time.Now().Add(time.Hour)

	serverCount, totalTools := ds.GetCacheStats()
	if serverCount != 2 {
		t.Errorf("expected 2 servers, got %d", serverCount)
	}
	if totalTools != 3 {
		t.Errorf("expected 3 tools, got %d", totalTools)
	}
}

func TestToolDiscoveryService_GetCacheStats_Empty(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	serverCount, totalTools := ds.GetCacheStats()
	if serverCount != 0 {
		t.Errorf("expected 0 servers, got %d", serverCount)
	}
	if totalTools != 0 {
		t.Errorf("expected 0 tools, got %d", totalTools)
	}
}

func TestToolDiscoveryService_GetToolsByServer(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	ds.cache["myserver"] = []DiscoveredTool{{Name: "tool1", ServerName: "myserver"}}

	tools, ok := ds.GetToolsByServer("myserver")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
}

func TestToolDiscoveryService_GetToolByName_Found(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	ds.cache["srv"] = []DiscoveredTool{{Name: "find_me", ServerName: "srv"}}

	tool, ok := ds.GetToolByName("find_me")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tool.Name != "find_me" {
		t.Errorf("expected name find_me, got %s", tool.Name)
	}
}

func TestToolDiscoveryService_GetAllTools(t *testing.T) {
	ds := NewToolDiscoveryService(15 * time.Minute)
	ds.cache["a"] = []DiscoveredTool{{Name: "t1", ServerName: "a"}}
	ds.cache["b"] = []DiscoveredTool{{Name: "t2", ServerName: "b"}}
	ds.cacheExpiry["a"] = time.Now().Add(time.Hour)
	ds.cacheExpiry["b"] = time.Now().Add(time.Hour)

	tools := ds.GetAllTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestParseInputSchema_Nil(t *testing.T) {
	result := parseInputSchema(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestParseInputSchema_ValidMap(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}
	result := parseInputSchema(schema)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != "object" {
		t.Errorf("expected type object, got %s", result.Type)
	}
}

func TestDiscoveredTool_ZeroValue(t *testing.T) {
	dt := DiscoveredTool{}
	// Verify struct fields are accessible with zero values
	if dt.Name != "" {
		t.Errorf("expected empty Name, got %s", dt.Name)
	}
	if dt.ServerName != "" {
		t.Errorf("expected empty ServerName, got %s", dt.ServerName)
	}
}

func TestInputSchema_ZeroValue(t *testing.T) {
	schema := InputSchema{}
	if schema.Type != "" {
		t.Errorf("expected empty Type, got %s", schema.Type)
	}
	if schema.Properties != nil {
		t.Error("expected nil Properties")
	}
}

func TestToolDiscoveryService_CacheExpiry(t *testing.T) {
	ds := NewToolDiscoveryService(1 * time.Millisecond)
	ds.cache["srv"] = []DiscoveredTool{{Name: "old_tool", ServerName: "srv"}}
	ds.cacheExpiry["srv"] = time.Now().Add(-1 * time.Second) // Already expired

	// DiscoverTools should NOT use cache since it's expired
	// Since we can't easily mock the MCP client, we verify the cache is still there
	// but the DiscoverTools method would attempt a refresh.
	// Here we just test GetTools returns the cached (expired) value:
	tools, ok := ds.GetTools("srv")
	if !ok {
		t.Fatal("expected cached value")
	}
	if len(tools) != 1 || tools[0].Name != "old_tool" {
		t.Error("expected old_tool in cache")
	}
}
