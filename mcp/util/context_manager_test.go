package util

import (
	"testing"
	"time"
)

func TestNewContextManager(t *testing.T) {
	cm := NewContextManager()
	if cm == nil {
		t.Fatal("NewContextManager() returned nil")
	}
	if cm.Size() != 0 {
		t.Errorf("expected size 0, got %d", cm.Size())
	}
}

func TestContextManager_SetAndGet(t *testing.T) {
	cm := NewContextManager()
	if err := cm.Set("key1", "value1", "test"); err != nil {
		t.Fatal(err)
	}
	val, ok := cm.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestContextManager_Get_NonExistent(t *testing.T) {
	cm := NewContextManager()
	_, ok := cm.Get("missing")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestContextManager_GetString(t *testing.T) {
	cm := NewContextManager()
	cm.Set("str", "hello", "test")

	val, ok := cm.GetString("str")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "hello" {
		t.Errorf("expected hello, got %s", val)
	}
}

func TestContextManager_GetString_WrongType(t *testing.T) {
	cm := NewContextManager()
	cm.Set("num", 42, "test")

	_, ok := cm.GetString("num")
	if ok {
		t.Error("expected ok=false for wrong type")
	}
}

func TestContextManager_GetInt(t *testing.T) {
	cm := NewContextManager()
	cm.Set("count", 99, "test")

	val, ok := cm.GetInt("count")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != 99 {
		t.Errorf("expected 99, got %d", val)
	}
}

func TestContextManager_GetInt_WrongType(t *testing.T) {
	cm := NewContextManager()
	cm.Set("name", "test", "test")

	_, ok := cm.GetInt("name")
	if ok {
		t.Error("expected ok=false for wrong type")
	}
}

func TestContextManager_GetMap(t *testing.T) {
	cm := NewContextManager()
	data := map[string]any{"a": 1, "b": 2}
	cm.Set("map", data, "test")

	val, ok := cm.GetMap("map")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val["a"] != 1 {
		t.Errorf("expected a=1, got %v", val["a"])
	}
}

func TestContextManager_GetMap_WrongType(t *testing.T) {
	cm := NewContextManager()
	cm.Set("str", "not a map", "test")

	_, ok := cm.GetMap("str")
	if ok {
		t.Error("expected ok=false for wrong type")
	}
}

func TestContextManager_Has(t *testing.T) {
	cm := NewContextManager()
	cm.Set("exists", true, "test")

	if !cm.Has("exists") {
		t.Error("expected Has to return true")
	}
	if cm.Has("missing") {
		t.Error("expected Has to return false for missing key")
	}
}

func TestContextManager_Clear(t *testing.T) {
	cm := NewContextManager()
	cm.Set("a", 1, "test")
	cm.Set("b", 2, "test")

	cm.Clear()
	if cm.Size() != 0 {
		t.Errorf("expected size 0 after Clear, got %d", cm.Size())
	}
}

func TestContextManager_Delete(t *testing.T) {
	cm := NewContextManager()
	cm.Set("a", 1, "test")
	cm.Set("b", 2, "test")

	cm.Delete("a")
	if cm.Has("a") {
		t.Error("expected key a to be deleted")
	}
	if !cm.Has("b") {
		t.Error("expected key b to still exist")
	}
}

func TestContextManager_DeletePrefix(t *testing.T) {
	cm := NewContextManager()
	cm.Set("stitch_project", "p1", "test")
	cm.Set("stitch_screen", "s1", "test")
	cm.Set("other_key", "x", "test")

	count := cm.DeletePrefix("stitch_")
	if count != 2 {
		t.Errorf("expected to delete 2 keys, got %d", count)
	}
	if cm.Has("stitch_project") {
		t.Error("expected stitch_project to be deleted")
	}
	if !cm.Has("other_key") {
		t.Error("expected other_key to still exist")
	}
}

func TestContextManager_GetAll(t *testing.T) {
	cm := NewContextManager()
	cm.Set("a", 1, "test")
	cm.Set("b", 2, "test")

	all := cm.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 items, got %d", len(all))
	}
}

func TestContextManager_GetKeys(t *testing.T) {
	cm := NewContextManager()
	cm.Set("x", 1, "test")
	cm.Set("y", 2, "test")

	keys := cm.GetKeys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestContextManager_ToJSON_And_FromJSON(t *testing.T) {
	cm := NewContextManager()
	cm.Set("name", "test", "source")
	cm.Set("count", 42, "source")

	jsonStr, err := cm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error = %v", err)
	}

	cm2 := NewContextManager()
	if err := cm2.FromJSON(jsonStr); err != nil {
		t.Fatalf("FromJSON error = %v", err)
	}

	val, ok := cm2.Get("name")
	if !ok {
		t.Fatal("expected name to exist after FromJSON")
	}
	if val != "test" {
		t.Errorf("expected name=test, got %v", val)
	}
}

func TestContextManager_FromJSON_InvalidJSON(t *testing.T) {
	cm := NewContextManager()
	err := cm.FromJSON("not valid json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestContextManager_UpdatedAt(t *testing.T) {
	cm := NewContextManager()
	cm.Set("key", "val", "test")

	all := cm.GetAll()
	cv := all["key"]
	if cv.UpdatedAt == "" {
		t.Error("expected UpdatedAt to be set")
	}
	// Verify it's a valid RFC3339 timestamp
	_, err := time.Parse(time.RFC3339, cv.UpdatedAt)
	if err != nil {
		t.Errorf("expected UpdatedAt to be RFC3339, got %q", cv.UpdatedAt)
	}
}

func TestContextManager_Source(t *testing.T) {
	cm := NewContextManager()
	cm.Set("key", "val", "my-tool")

	all := cm.GetAll()
	cv := all["key"]
	if cv.Source != "my-tool" {
		t.Errorf("expected source my-tool, got %q", cv.Source)
	}
}

func TestContextManager_Overwrite(t *testing.T) {
	cm := NewContextManager()
	cm.Set("key", "old", "source1")
	cm.Set("key", "new", "source2")

	val, ok := cm.Get("key")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "new" {
		t.Errorf("expected new value, got %v", val)
	}

	all := cm.GetAll()
	if all["key"].Source != "source2" {
		t.Errorf("expected source2 after overwrite, got %q", all["key"].Source)
	}
}
