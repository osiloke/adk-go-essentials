package infrastructure

import (
	"strings"
	"testing"

	"google.golang.org/adk/model"
)

func TestModelInfo_ZeroValue(t *testing.T) {
	m := ModelInfo{}
	if m.ID != "" {
		t.Errorf("expected empty ID, got %s", m.ID)
	}
}

func TestStaticModels_Defined(t *testing.T) {
	if len(STATIC_MODELS) == 0 {
		t.Fatal("STATIC_MODELS should not be empty")
	}
}

func TestStaticModels_HasGemini(t *testing.T) {
	found := false
	for _, m := range STATIC_MODELS {
		if m.Provider == "gemini" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one gemini model")
	}
}

func TestStaticModels_HasDeepSeek(t *testing.T) {
	found := false
	for _, m := range STATIC_MODELS {
		if m.Provider == "deepseek" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one deepseek model")
	}
}

func TestStaticModels_HasOpenAI(t *testing.T) {
	found := false
	for _, m := range STATIC_MODELS {
		if m.Provider == "openai" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one openai model")
	}
}

func TestModelFactory_NilWhenNoModels(t *testing.T) {
	f := &ModelFactory{
		availableModels: []*ModelInfo{},
		modelInstances:  make(map[string]model.LLM),
	}

	_, err := f.GetModelByProfile("balanced")
	if err == nil {
		t.Fatal("expected error when no models available")
	}
}

func TestModelFactory_GetModelByID_NotFound(t *testing.T) {
	f := &ModelFactory{
		modelInstances: make(map[string]model.LLM),
	}

	_, err := f.GetModelByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %v", err)
	}
}

func TestModelFactory_GetModelByProfile_Nil(t *testing.T) {
	f := &ModelFactory{
		availableModels: []*ModelInfo{},
		modelInstances:  make(map[string]model.LLM),
	}

	_, err := f.GetModelByProfile("balanced")
	if err == nil {
		t.Fatal("expected error when no models available")
	}
}

func TestGetScoreFn_Quick(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("quick")

	fastLow := &ModelInfo{Speed: "fast", Cost: "low", Priority: 5}
	slowHigh := &ModelInfo{Speed: "slow", Cost: "high", Priority: 5}

	if fn(fastLow) <= fn(slowHigh) {
		t.Error("fast+low should score higher than slow+high for quick profile")
	}
}

func TestGetScoreFn_Fast(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("fast")

	fast := &ModelInfo{Speed: "fast", Priority: 5}
	balanced := &ModelInfo{Speed: "balanced", Priority: 5}

	if fn(fast) <= fn(balanced) {
		t.Error("fast should score higher than balanced for fast profile")
	}
}

func TestGetScoreFn_Smart(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("smart")

	high := &ModelInfo{Intelligence: "high", Cost: "low", Priority: 5}
	medium := &ModelInfo{Intelligence: "medium", Cost: "low", Priority: 5}

	if fn(high) <= fn(medium) {
		t.Error("high intelligence should score higher for smart profile")
	}
}

func TestGetScoreFn_Reasoning(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("reasoning")

	highPlus := &ModelInfo{Intelligence: "high+", Priority: 5}
	high := &ModelInfo{Intelligence: "high", Priority: 5}

	if fn(highPlus) <= fn(high) {
		t.Error("high+ should score higher than high for reasoning profile")
	}
}

func TestGetScoreFn_Balanced_Default(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("unknown_profile")

	balanced := &ModelInfo{Speed: "balanced", Intelligence: "high", Priority: 5}
	slowLow := &ModelInfo{Speed: "slow", Intelligence: "low", Priority: 5}

	if fn(balanced) <= fn(slowLow) {
		t.Error("balanced+high should score higher than slow+low for default profile")
	}
}

func TestGetScoreFn_CaseInsensitive(t *testing.T) {
	f := &ModelFactory{}
	fn1 := f.getScoreFn("SMART")
	fn2 := f.getScoreFn("smart")

	m := &ModelInfo{Intelligence: "high", Cost: "low", Priority: 5}
	if fn1(m) != fn2(m) {
		t.Error("profile matching should be case insensitive")
	}
}

func TestGetScoreFn_Embedding(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("embedding")

	embedding := &ModelInfo{ID: "gemini/text-embedding-001", Priority: 5}
	normal := &ModelInfo{ID: "gemini/gemini-2.0-flash", Priority: 5}

	if fn(embedding) <= fn(normal) {
		t.Error("embedding model should score higher for embedding profile")
	}
}

func TestModelFactory_PrintSummary_DoesNotPanic(t *testing.T) {
	f := &ModelFactory{
		availableModels: []*ModelInfo{
			{ID: "gemini/gemini-2.0-flash", Provider: "gemini", Label: "Flash", Cost: "low", Intelligence: "medium", Speed: "fast", SupportsTools: true, Priority: 10},
		},
		modelInstances: make(map[string]model.LLM),
	}

	// Should not panic
	f.PrintSummary()
}

func TestModelFactory_PrintSummary_Empty(t *testing.T) {
	f := &ModelFactory{
		availableModels: []*ModelInfo{},
		modelInstances:  make(map[string]model.LLM),
	}

	// Should not panic even with no models
	f.PrintSummary()
}

func TestModelInfo_AllFieldsSettable(t *testing.T) {
	m := ModelInfo{
		ID:            "test/id",
		Provider:      "test",
		Label:         "Test Model",
		Cost:          "medium",
		Intelligence:  "high",
		Speed:         "balanced",
		SupportsTools: true,
		Premium:       true,
		Priority:      15,
	}
	if m.ID != "test/id" {
		t.Errorf("expected ID test/id, got %s", m.ID)
	}
	if !m.SupportsTools {
		t.Error("expected SupportsTools to be true")
	}
	if !m.Premium {
		t.Error("expected Premium to be true")
	}
}

func TestStaticModels_AllHaveRequiredFields(t *testing.T) {
	for _, m := range STATIC_MODELS {
		if m.ID == "" {
			t.Error("model has empty ID")
		}
		if m.Provider == "" {
			t.Error("model has empty Provider")
		}
		if m.Label == "" {
			t.Error("model has empty Label")
		}
		if m.Cost == "" {
			t.Error("model has empty Cost")
		}
		if m.Intelligence == "" {
			t.Error("model has empty Intelligence")
		}
		if m.Speed == "" {
			t.Error("model has empty Speed")
		}
	}
}

func TestStaticModels_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, m := range STATIC_MODELS {
		if seen[m.ID] {
			t.Errorf("duplicate model ID: %s", m.ID)
		}
		seen[m.ID] = true
	}
}

func TestGetScoreFn_Fast_BalancedSpeed(t *testing.T) {
	f := &ModelFactory{}
	fn := f.getScoreFn("fast")

	// Only "fast" speed gets 100 points; balanced and slow both get 0 from speed,
	// so they are equal in the speed component for the fast profile.
	balanced := &ModelInfo{Speed: "balanced", Priority: 10}
	slow := &ModelInfo{Speed: "slow", Priority: 10}

	if fn(balanced) != fn(slow) {
		t.Errorf("balanced and slow should score equally for fast profile (both 0 from speed): balanced=%d, slow=%d", fn(balanced), fn(slow))
	}
}
