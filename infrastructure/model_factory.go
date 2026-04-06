package infrastructure

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/osiloke/adk-go-essentials/observability"

	"github.com/cohesion-org/deepseek-go"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// ModelInfo describes the capabilities of a specific LLM to allow dynamic profile selection.
type ModelInfo struct {
	ID            string // e.g., "gemini/gemini-2.0-flash"
	Provider      string // e.g., "gemini" or "deepseek"
	Label         string
	Cost          string // "low", "medium", "high"
	Intelligence  string // "low", "medium", "high", "high+"
	Speed         string // "fast", "balanced", "slow"
	SupportsTools bool
	Premium       bool
	Priority      int // Higher means preferred over others with similar stats
}

// ModelFactory manages dynamic model selection based on active environment API keys and requested capabilities.
type ModelFactory struct {
	availableModels []*ModelInfo
	modelInstances  map[string]model.LLM
}

// STATIC_MODELS defines the capabilities for known supported models.
var STATIC_MODELS = []*ModelInfo{
	// Gemini Models
	{
		ID:            "gemini/gemini-2.0-flash",
		Provider:      "gemini",
		Label:         "Gemini 2.0 Flash",
		Cost:          "low",
		Intelligence:  "medium",
		Speed:         "fast",
		SupportsTools: true,
		Priority:      10,
	},
	{
		ID:            "gemini/gemini-2.0-pro-exp-02-05", // Example using experimental pro model for higher reasoning
		Provider:      "gemini",
		Label:         "Gemini 2.0 Pro",
		Cost:          "medium",
		Intelligence:  "high",
		Speed:         "balanced",
		SupportsTools: true,
		Premium:       true,
		Priority:      10,
	},

	// DeepSeek Models (Native via cohesion-org/deepseek-go)
	{
		ID:            "deepseek/" + deepseek.DeepSeekChat,
		Provider:      "deepseek",
		Label:         "DeepSeek V3 (Chat)",
		Cost:          "low",
		Intelligence:  "high",
		Speed:         "fast",
		SupportsTools: true,
		Priority:      9,
	},
	{
		ID:            "deepseek/" + deepseek.DeepSeekReasoner,
		Provider:      "deepseek",
		Label:         "DeepSeek R1 (Reasoner)",
		Cost:          "medium",
		Intelligence:  "high+",
		Speed:         "slow",
		SupportsTools: true,
		Premium:       true,
		Priority:      10,
	},

	// DeepSeek Models (OpenAI Compatible via api.deepseek.com)
	{
		ID:            "deepseek-openai/" + deepseek.DeepSeekChat,
		Provider:      "deepseek-openai",
		Label:         "DeepSeek V3 (Chat) [OpenAI-API]",
		Cost:          "low",
		Intelligence:  "high",
		Speed:         "fast",
		SupportsTools: true,
		Priority:      9,
	},
	{
		ID:            "deepseek-openai/" + deepseek.DeepSeekReasoner,
		Provider:      "deepseek-openai",
		Label:         "DeepSeek R1 (Reasoner) [OpenAI-API]",
		Cost:          "medium",
		Intelligence:  "high+",
		Speed:         "slow",
		SupportsTools: true,
		Premium:       true,
		Priority:      10,
	},

	// OpenAI Models (e.g. standard GPT-4o)
	{
		ID:            "openai/gpt-4o",
		Provider:      "openai",
		Label:         "OpenAI GPT-4o",
		Cost:          "medium",
		Intelligence:  "high",
		Speed:         "balanced",
		SupportsTools: true,
		Premium:       true,
		Priority:      9,
	},
	{
		ID:            "openai/gpt-4o-mini",
		Provider:      "openai",
		Label:         "OpenAI GPT-4o-Mini",
		Cost:          "low",
		Intelligence:  "medium",
		Speed:         "fast",
		SupportsTools: true,
		Premium:       false,
		Priority:      9,
	},

	// OpenRouter Models
	{
		ID:            "openrouter/anthropic/claude-3.5-sonnet",
		Provider:      "openrouter",
		Label:         "Claude 3.5 Sonnet (OpenRouter)",
		Cost:          "medium",
		Intelligence:  "high",
		Speed:         "balanced",
		SupportsTools: true,
		Premium:       true,
		Priority:      11, // Very high priority if available
	},
	{
		ID:            "openrouter/meta-llama/llama-3.3-70b-instruct",
		Provider:      "openrouter",
		Label:         "Llama 3.3 70B (OpenRouter)",
		Cost:          "low",
		Intelligence:  "medium",
		Speed:         "fast",
		SupportsTools: true,
		Premium:       false,
		Priority:      8,
	},
}

// InitProviders discovers available platforms via environment variables and instantiates the models.
func InitProviders(ctx context.Context) (*ModelFactory, error) {
	factory := &ModelFactory{
		availableModels: []*ModelInfo{},
		modelInstances:  make(map[string]model.LLM),
	}

	hasGemini := os.Getenv("GOOGLE_API_KEY") != ""
	hasDeepSeek := os.Getenv("DEEPSEEK_API_KEY") != ""
	hasDeepSeekOpenAI := os.Getenv("DEEPSEEK_OPENAI_KEY") != ""
	hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""

	// Instantiate capabilities and actual models for enabled providers
	for _, m := range STATIC_MODELS {
		if m.Provider == "gemini" && hasGemini {
			geminiModel, err := gemini.NewModel(ctx, strings.TrimPrefix(m.ID, "gemini/"), &genai.ClientConfig{
				APIKey: os.Getenv("GOOGLE_API_KEY"),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to init %s: %w", m.ID, err)
			}
			factory.modelInstances[m.ID] = NewRetryModel(geminiModel, 3, 2*time.Second, 30*time.Second)
			factory.availableModels = append(factory.availableModels, m)
		} else if m.Provider == "deepseek" && hasDeepSeek {
			dsModel, err := NewModel(os.Getenv("DEEPSEEK_API_KEY"), strings.TrimPrefix(m.ID, "deepseek/"))
			if err != nil {
				return nil, fmt.Errorf("failed to init %s: %w", m.ID, err)
			}
			factory.modelInstances[m.ID] = NewRetryModel(dsModel, 3, 2*time.Second, 30*time.Second)
			factory.availableModels = append(factory.availableModels, m)
		} else if m.Provider == "deepseek-openai" && hasDeepSeekOpenAI {
			baseURL := "https://api.deepseek.com"
			dsModel, err := NewOpenAIModel(os.Getenv("DEEPSEEK_OPENAI_KEY"), baseURL, strings.TrimPrefix(m.ID, "deepseek-openai/"))
			if err != nil {
				return nil, fmt.Errorf("failed to init %s: %w", m.ID, err)
			}
			factory.modelInstances[m.ID] = NewRetryModel(dsModel, 3, 2*time.Second, 30*time.Second)
			factory.availableModels = append(factory.availableModels, m)
		} else if m.Provider == "openai" && hasOpenAI {
			baseURL := os.Getenv("OPENAI_BASE_URL")
			oaiModel, err := NewOpenAIModel(os.Getenv("OPENAI_API_KEY"), baseURL, strings.TrimPrefix(m.ID, "openai/"))
			if err != nil {
				return nil, fmt.Errorf("failed to init %s: %w", m.ID, err)
			}
			factory.modelInstances[m.ID] = NewRetryModel(oaiModel, 3, 2*time.Second, 30*time.Second)
			factory.availableModels = append(factory.availableModels, m)
		} else if m.Provider == "openrouter" && os.Getenv("OPENROUTER_API_KEY") != "" {
			baseURL := "https://openrouter.ai/api/v1"
			oaiModel, err := NewOpenAIModel(os.Getenv("OPENROUTER_API_KEY"), baseURL, strings.TrimPrefix(m.ID, "openrouter/"))
			if err != nil {
				return nil, fmt.Errorf("failed to init %s: %w", m.ID, err)
			}
			factory.modelInstances[m.ID] = NewRetryModel(oaiModel, 3, 2*time.Second, 30*time.Second)
			factory.availableModels = append(factory.availableModels, m)
		}
	}

	if len(factory.availableModels) == 0 {
		return nil, fmt.Errorf("no active LLM providers configured. Please set GOOGLE_API_KEY or DEEPSEEK_API_KEY")
	}

	factory.PrintSummary()

	return factory, nil
}

// getScoreFn returns a scoring function for a given profile
func (f *ModelFactory) getScoreFn(profile string) func(m *ModelInfo) int {
	profile = strings.ToLower(profile)
	switch profile {
	case "quick":
		return func(m *ModelInfo) int {
			score := 0
			if m.Speed == "fast" {
				score += 100
			}
			if m.Cost == "low" {
				score += 50
			}
			return score + m.Priority
		}
	case "fast":
		// Fast profile: Prefer speed == fast
		return func(m *ModelInfo) int {
			score := 0
			if m.Speed == "fast" {
				score += 100
			}
			return score + m.Priority
		}
	case "smart":
		// Smart profile: Prefer intelligence high/high+, then cost
		return func(m *ModelInfo) int {
			score := 0
			switch m.Intelligence {
			case "high+":
				score += 100
			case "high":
				score += 80
			}
			if m.Cost == "low" {
				score += 10
			}
			return score + m.Priority
		}
	case "reasoning":
		// Reasoning: explicitly for high+ intelligence code/math
		return func(m *ModelInfo) int {
			score := 0
			if m.Intelligence == "high+" {
				score += 100
			}
			return score + m.Priority
		}
	case "embedding":
		return func(m *ModelInfo) int {
			score := 0
			if strings.Contains(strings.ToLower(m.ID), "embedding") {
				score += 100
			}
			return score + m.Priority
		}
	case "balanced":
		fallthrough
	default:
		// Balanced default profile
		return func(m *ModelInfo) int {
			score := 0
			if m.Speed == "balanced" || m.Speed == "fast" {
				score += 50
			}
			if m.Intelligence == "medium" || m.Intelligence == "high" {
				score += 50
			}
			return score + m.Priority
		}
	}
}

// GetModelByProfile returns the best active model matching the required capability profile.
func (f *ModelFactory) GetModelByProfile(profile string) (model.LLM, error) {
	if len(f.availableModels) == 0 {
		return nil, fmt.Errorf("no active models available")
	}

	pool := make([]*ModelInfo, len(f.availableModels))
	copy(pool, f.availableModels)

	scoreFn := f.getScoreFn(profile)

	// Sort available pool descending by score, tiebreak by original struct priority
	sort.Slice(pool, func(i, j int) bool {
		return scoreFn(pool[i]) > scoreFn(pool[j])
	})

	bestModel := pool[0] // Return top result

	// Print logging for observability in development context
	observability.Log.Infof("[Model Factory] Selected %s (%s) for profile '%s'", bestModel.ID, bestModel.Label, profile)

	return f.modelInstances[bestModel.ID], nil
}

// GetOriginalModel explicitly gets a model by its direct ID (e.g. gemini/gemini-2.0-flash)
func (f *ModelFactory) GetModelByID(id string) (model.LLM, error) {
	if m, ok := f.modelInstances[id]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("model %q not found or not configured", id)
}

// PrintSummary prints a summary of the loaded models and their rankings for different profiles.
func (f *ModelFactory) PrintSummary() {
	observability.Log.Infof("\n[Model Factory] Initializing Model Factory...\n" +
		"=============================================================\n" +
		"               ADK ESSENTIALS MODEL FACTORY                 \n" +
		"===============================================================")

	providersMap := make(map[string]bool)
	for _, m := range f.availableModels {
		providersMap[m.Provider] = true
	}
	var providers []string
	for p := range providersMap {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	observability.Log.Infof("[Providers] %s\n[Models]    %d loaded\n---------------------------------------------------------------\n[Profiles]", strings.Join(providers, ", "), len(f.availableModels))

	profiles := []string{"QUICK", "FAST", "BALANCED", "SMART", "REASONING", "EMBEDDING"}
	for _, profile := range profiles {
		observability.Log.Infof("> %s", profile)

		pool := make([]*ModelInfo, len(f.availableModels))
		copy(pool, f.availableModels)

		scoreFn := f.getScoreFn(profile)
		sort.SliceStable(pool, func(i, j int) bool {
			return scoreFn(pool[i]) > scoreFn(pool[j])
		})

		// Filter out those with score <= 0 if they don't match, unless no one matches
		var candidates []*ModelInfo
		for _, m := range pool {
			if scoreFn(m) > 0 {
				candidates = append(candidates, m)
			}
		}
		if len(candidates) == 0 && len(pool) > 0 {
			candidates = append(candidates, pool[0])
		}

		if len(candidates) > 0 {
			selected := candidates[0]
			var runRunnerUp string
			if len(candidates) > 1 {
				runRunnerUp = fmt.Sprintf("\n  Runner up:    %s", candidates[1].ID)
			}

			// Find premium
			var premium *ModelInfo
			for _, m := range candidates {
				if m.Premium {
					premium = m
					break
				}
			}

			var premID string
			if premium != nil {
				premID = premium.ID
			} else {
				premID = selected.ID
			}

			var candidateIDs []string
			for _, m := range candidates {
				candidateIDs = append(candidateIDs, m.ID)
			}

			observability.Log.Infof("  Selected:     %s%s\n  Premium:      %s\n  Candidates: %s", selected.ID, runRunnerUp, premID, strings.Join(candidateIDs, ", "))
		} else {
			observability.Log.Infof("  Selected:   none")
		}
	}
	observability.Log.Infof("==================================================")
}
