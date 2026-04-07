package state

import (
	"encoding/json"
	"testing"
)

func TestParseAutoCoTResult_NilValue(t *testing.T) {
	_, err := ParseAutoCoTResult(nil)
	if err == nil {
		t.Fatal("expected error for nil value, got nil")
	}
}

func TestParseAutoCoTResult_ValidJSONString(t *testing.T) {
	input := `{"status": "READY", "execution_plan": {"steps": ["step1"]}}`
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "READY" {
		t.Errorf("expected status READY, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_MarkdownJSONBlock(t *testing.T) {
	input := "```json\n{\"status\": \"READY\", \"missing_info\": []}\n```"
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "READY" {
		t.Errorf("expected status READY, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_NonJSONString_Fallback(t *testing.T) {
	input := "Sure, I'll help you with that. Here's what I think..."
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() returned error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO fallback, got %v", result["status"])
	}
	if result["reasoning_trace"] != input {
		t.Errorf("expected reasoning_trace to contain raw input")
	}
}

func TestParseAutoCoTResult_MapInput(t *testing.T) {
	input := map[string]any{"status": "DONE", "count": 42}
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "DONE" {
		t.Errorf("expected status DONE, got %v", result["status"])
	}
	if result["count"] != 42 {
		t.Errorf("expected count 42, got %v", result["count"])
	}
}

func TestParseAutoCoTResult_InvalidType(t *testing.T) {
	_, err := ParseAutoCoTResult(123)
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}

func TestParseAutoCoTResult_ConversationalNoise(t *testing.T) {
	input := "Here's the result: {\"status\": \"OK\"}"
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "OK" {
		t.Errorf("expected status OK, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_BracesOutOfOrder(t *testing.T) {
	input := "}{"
	result, err := ParseAutoCoTResult(input)
	// The code finds { at index 1 and } at index 0, but end > start check fails,
	// so it tries to parse the original string which is invalid JSON,
	// resulting in NEEDS_INFO fallback (not an error).
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() returned error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO fallback, got %v", result["status"])
	}
}

func TestCommonKeys_Defined(t *testing.T) {
	if len(CommonKeys) == 0 {
		t.Error("CommonKeys should not be empty")
	}
}

func TestDefaultKeys_IncludesCommon(t *testing.T) {
	for _, key := range CommonKeys {
		found := false
		for _, dk := range DefaultKeys {
			if dk == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultKeys should include CommonKey %q", key)
		}
	}
}

func TestParseAutoCoTResult_ExecutionPlanExtraction(t *testing.T) {
	input := map[string]any{
		"execution_plan": map[string]any{"phases": []string{"design", "build"}},
		"metadata":       "extra",
	}
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	plan, ok := result["execution_plan"]
	if !ok {
		t.Fatal("expected execution_plan in result")
	}
	planMap := plan.(map[string]any)
	phases := planMap["phases"].([]string)
	if len(phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(phases))
	}
}

func TestMapAutoCoTResult_NilState(t *testing.T) {
	// This test would require mocking the CallbackContext and State interfaces.
	// Since those are ADK interfaces, we test the ParseAutoCoTResult logic thoroughly
	// and trust MapAutoCoTResult simply delegates.
}

func TestParseAutoCoTResult_EmptyString(t *testing.T) {
	result, err := ParseAutoCoTResult("")
	// Empty string triggers NEEDS_INFO fallback (not an error)
	if err != nil {
		t.Fatalf("expected NEEDS_INFO fallback, got error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_EmptyObject(t *testing.T) {
	result, err := ParseAutoCoTResult("{}")
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestNEEDS_INFOFallback_HasAllFields(t *testing.T) {
	result, _ := ParseAutoCoTResult("not json at all")
	requiredFields := []string{"status", "reasoning_trace", "missing_info"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("NEEDS_INFO fallback missing field %q", field)
		}
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO status, got %v", result["status"])
	}
	if missingInfo, ok := result["missing_info"].([]any); !ok || len(missingInfo) == 0 {
		t.Error("missing_info should be a non-empty array")
	}
}

func TestParseAutoCoTResult_NestedJSON(t *testing.T) {
	input := `{
		"status": "READY",
		"execution_plan": {
			"use_cases": [
				{"name": "UC1", "priority": "high"},
				{"name": "UC2", "priority": "low"}
			]
		},
		"glossary": {"entries": 5}
	}`
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	plan := result["execution_plan"].(map[string]any)
	useCases := plan["use_cases"].([]any)
	if len(useCases) != 2 {
		t.Errorf("expected 2 use cases, got %d", len(useCases))
	}
}

func TestParseAutoCoTResult_MarkdownWithoutJSON(t *testing.T) {
	input := "```python\nprint('hello')\n```"
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() returned error = %v", err)
	}
	// Should fall back to NEEDS_INFO since there's no valid JSON
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO fallback, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_LargeInput(t *testing.T) {
	largeJSON := `{"status": "READY", "data": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	result, err := ParseAutoCoTResult(largeJSON)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "READY" {
		t.Errorf("expected status READY, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_InvalidJSONInsideMarkdown(t *testing.T) {
	input := "```json\n{invalid}\n```"
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() returned error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO fallback, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_ArrayInsteadOfObject(t *testing.T) {
	input := `["a", "b", "c"]`
	result, err := ParseAutoCoTResult(input)
	// Arrays unmarshal to []any which is not map[string]any, so NEEDS_INFO fallback
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() returned error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO fallback for array, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_NumberTypes(t *testing.T) {
	input := `{"count": 42, "ratio": 3.14, "flag": true}`
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	// JSON numbers become float64 in Go
	if result["count"] != float64(42) {
		t.Errorf("expected count 42, got %v", result["count"])
	}
}

func TestParseAutoCoTResult_NullValue(t *testing.T) {
	_, err := ParseAutoCoTResult(nil)
	if err == nil {
		t.Fatal("expected error for nil, got nil")
	}
}

func TestParseAutoCoTResult_WhitespaceOnly(t *testing.T) {
	result, err := ParseAutoCoTResult("   \n\t  ")
	// Whitespace triggers NEEDS_INFO fallback (not an error)
	if err != nil {
		t.Fatalf("expected NEEDS_INFO fallback, got error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_EmbeddedNewlines(t *testing.T) {
	input := "{\n  \"status\": \"OK\",\n  \"msg\": \"hello\\nworld\"\n}"
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "OK" {
		t.Errorf("expected status OK, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_UnicodeContent(t *testing.T) {
	input := `{"status": "OK", "message": "你好世界 🎉"}`
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["message"] != "你好世界 🎉" {
		t.Errorf("expected unicode message, got %v", result["message"])
	}
}

func TestParseAutoCoTResult_DuplicateKeys(t *testing.T) {
	input := `{"status": "first", "status": "second"}`
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	// Last key wins in JSON unmarshaling
	if result["status"] != "second" {
		t.Errorf("expected status 'second' (last wins), got %v", result["status"])
	}
}

func TestParseAutoCoTResult_PartialJSON(t *testing.T) {
	input := `{"status": "READY"`
	result, err := ParseAutoCoTResult(input)
	// Partial JSON triggers NEEDS_INFO fallback (not an error)
	if err != nil {
		t.Fatalf("expected NEEDS_INFO fallback, got error = %v", err)
	}
	if result["status"] != "NEEDS_INFO" {
		t.Errorf("expected NEEDS_INFO, got %v", result["status"])
	}
}

func TestParseAutoCoTResult_MultipleMarkdownBlocks(t *testing.T) {
	input := "Some text ```json\n{\"status\": \"A\"}\n``` more text ```json\n{\"status\": \"B\"}\n```"
	result, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	// First regex match should be extracted
	if result["status"] != "A" {
		t.Errorf("expected status A (first match), got %v", result["status"])
	}
}

func TestMapAutoCoTResult_Flow(t *testing.T) {
	// Test ParseAutoCoTResult used within MapAutoCoTResult
	input := `{"execution_plan": {"phase": "build"}}`
	resultMap, err := ParseAutoCoTResult(input)
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if _, ok := resultMap["execution_plan"]; !ok {
		t.Error("expected execution_plan to be extractable")
	}
}

func TestParseAutoCoTResult_Roundtrip(t *testing.T) {
	original := map[string]any{
		"status": "READY",
		"data": map[string]any{
			"items": []any{"a", "b", "c"},
		},
	}
	data, _ := json.Marshal(original)
	result, err := ParseAutoCoTResult(string(data))
	if err != nil {
		t.Fatalf("ParseAutoCoTResult() error = %v", err)
	}
	if result["status"] != "READY" {
		t.Errorf("expected status READY after roundtrip, got %v", result["status"])
	}
}
