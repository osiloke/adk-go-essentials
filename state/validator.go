package state

import (
	"context"
	"fmt"
	"iter"

	"github.com/osiloke/adk-go-essentials/observability"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// CheckRequiredKeysExist returns a BeforeAgentCallback that verifies the presence of required keys.
// If a required key is missing, it returns a Content response that tells the agent to stop.
// This approach works for both standalone agents and agents inside loops.
func CheckRequiredKeysExist(keys []string) agent.BeforeAgentCallback {
	return func(ctx agent.CallbackContext) (*genai.Content, error) {
		observability.Log.Infof("🔍 [%s] Verifying required state keys: %v", ctx.AgentName(), keys)
		state := ctx.State()
		if state == nil {
			return nil, fmt.Errorf("state is nil")
		}

		for _, k := range keys {
			val, err := state.Get(k)
			if err != nil || val == nil {
				observability.Log.Warnf("⚠️ [%s] Required key %q not found in state, stopping.", ctx.AgentName(), k)
				// Return Content instead of error - this properly stops the agent
				return &genai.Content{
					Parts: []*genai.Part{{Text: fmt.Sprintf("⚠️ Cannot proceed: required key %q not found in state. Please ensure the required data is available before running.", k)}},
				}, nil
			}
		}
		observability.Log.Infof("✅ [%s] All required keys present.", ctx.AgentName())
		return nil, nil
	}
}

// CheckRequiredArtifactsExist returns a BeforeAgentCallback that verifies the presence of required artifacts.
// If a required artifact is missing, it returns a Content response that tells the agent to stop.
// This approach works for both standalone agents and agents inside loops.
func CheckRequiredArtifactsExist(artifacts []string) agent.BeforeAgentCallback {
	return func(ctx agent.CallbackContext) (*genai.Content, error) {
		observability.Log.Infof("🔍 [%s] Verifying required artifacts: %v", ctx.AgentName(), artifacts)
		service := ctx.Artifacts()
		if service == nil {
			return nil, fmt.Errorf("artifacts service is nil")
		}

		for _, name := range artifacts {
			_, err := service.Load(context.Background(), name)
			if err != nil {
				observability.Log.Warnf("⚠️ [%s] Required artifact %q not found, stopping.", ctx.AgentName(), name)
				// Return Content instead of error - this properly stops the agent
				return &genai.Content{
					Parts: []*genai.Part{{Text: fmt.Sprintf("⚠️ Cannot proceed: required artifact %q not found. Please ensure the required file exists before running.", name)}},
				}, nil
			}
		}
		observability.Log.Infof("✅ [%s] All required artifacts present.", ctx.AgentName())
		return nil, nil
	}
}

// NewPreCheckAgent creates an agent that checks for required keys before running.
// Unlike callbacks, this agent can properly escalate to stop loop execution.
func NewPreCheckAgent(requiredKeys []string) (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        "PreCheck",
		Description: "Verifies required keys exist before proceeding.",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				state := ctx.Session().State()

				for _, k := range requiredKeys {
					val, err := state.Get(k)
					if err != nil || val == nil {
						observability.Log.Warnf("⚠️ [PreCheck] Required key %q not found in state, stopping loop.", k)
						yield(&session.Event{
							Author:  "PreCheck",
							Actions: session.EventActions{Escalate: true},
						}, nil)
						return
					}
				}

				observability.Log.Infof("✅ [PreCheck] All required keys present.")
				yield(&session.Event{
					Author: "PreCheck",
				}, nil)
			}
		},
	})
}

// LoadArtifactArgs defines arguments for loading an artifact into state.
type LoadArtifactArgs struct {
	ArtifactName string `json:"artifact_name" description:"The name of the artifact to load (e.g. 'use_case_1.md')"`
	StateKey     string `json:"state_key" description:"The state key to store the artifact content"`
}

// LoadArtifactToStateTool creates a tool that loads an artifact's content into session state.
// This is useful for making artifact content available as state for subsequent agents.
func LoadArtifactToStateTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "load_artifact_to_state",
		Description: "Loads an artifact's content into session state for use by other agents.",
	}, func(ctx tool.Context, args LoadArtifactArgs) (string, error) {
		observability.Log.Infof("🛠️ [TOOL CALL] load_artifact_to_state(artifact=%q, stateKey=%q)", args.ArtifactName, args.StateKey)

		artService := ctx.Artifacts()
		if artService == nil {
			return "", fmt.Errorf("artifact service is not available")
		}

		resp, err := artService.Load(context.Background(), args.ArtifactName)
		if err != nil {
			return "", fmt.Errorf("failed to load artifact %q: %w", args.ArtifactName, err)
		}

		if resp == nil || resp.Part == nil || resp.Part.InlineData == nil {
			return "", fmt.Errorf("artifact %q not found or has no content", args.ArtifactName)
		}

		content := string(resp.Part.InlineData.Data)

		state := ctx.State()
		if err := state.Set(args.StateKey, content); err != nil {
			return "", fmt.Errorf("failed to set state key %q: %w", args.StateKey, err)
		}

		observability.Log.Infof("✅ [TOOL] Loaded artifact %q into state key %q (%d bytes).", args.ArtifactName, args.StateKey, len(content))
		return fmt.Sprintf("Successfully loaded %q into state key %q", args.ArtifactName, args.StateKey), nil
	})
}

// PreCheckAndLoadArtifactConfig defines configuration for the PreCheckAndLoadArtifact callback.
type PreCheckAndLoadArtifactConfig struct {
	ArtifactName string // The artifact file to load (e.g., "autocot_plan.md")
	StateKey     string // The state key to store the content (e.g., "autocot_result")
	ErrorMessage string // Error message to return if artifact is not found
}

// PreCheckAndLoadArtifact creates a BeforeAgentCallback that:
// 1. Checks if the state key already exists
// 2. If not, loads the artifact and stores it in state
// 3. Returns an error message content if the artifact is not found
//
// This callback is designed to run BEFORE a loop starts, ensuring required artifacts are loaded.
func PreCheckAndLoadArtifact(cfg PreCheckAndLoadArtifactConfig) agent.BeforeAgentCallback {
	if cfg.ErrorMessage == "" {
		cfg.ErrorMessage = fmt.Sprintf("⚠️ Required artifact %q not found.", cfg.ArtifactName)
	}

	return func(ctx agent.CallbackContext) (*genai.Content, error) {
		observability.Log.Infof("🔍 [PreCheck] Verifying %s exists before starting loop", cfg.StateKey)
		state := ctx.State()
		if state == nil {
			return nil, fmt.Errorf("state is nil")
		}

		// Check if already in state
		val, err := state.Get(cfg.StateKey)
		if err == nil && val != nil {
			observability.Log.Infof("✅ [PreCheck] %s found in state", cfg.StateKey)
			return nil, nil
		}

		// Try to load from artifacts
		artService := ctx.Artifacts()
		if artService == nil {
			return nil, fmt.Errorf("artifact service is nil")
		}

		resp, err := artService.Load(context.Background(), cfg.ArtifactName)
		if err != nil {
			observability.Log.Warnf("⚠️ [PreCheck] Failed to load %s: %v", cfg.ArtifactName, err)
			return &genai.Content{
				Parts: []*genai.Part{{Text: cfg.ErrorMessage}},
			}, nil
		}

		if resp == nil || resp.Part == nil || resp.Part.InlineData == nil {
			observability.Log.Warnf("⚠️ [PreCheck] %s is empty or not found", cfg.ArtifactName)
			return &genai.Content{
				Parts: []*genai.Part{{Text: cfg.ErrorMessage}},
			}, nil
		}

		// Load into state
		content := string(resp.Part.InlineData.Data)
		if err := state.Set(cfg.StateKey, content); err != nil {
			return nil, fmt.Errorf("failed to set %s in state: %w", cfg.StateKey, err)
		}

		observability.Log.Infof("✅ [PreCheck] Loaded %s into state (%d bytes)", cfg.ArtifactName, len(content))
		return nil, nil
	}
}

// PreCheckAndLoadArtifactConfigMulti defines configuration for the PreCheckAndLoadArtifacts callback.
type PreCheckAndLoadArtifactConfigMulti struct {
	Artifacts     []PreCheckAndLoadArtifactConfig // List of artifact configurations to load
	StopOnFailure bool                            // If true, stops on first failure
	ErrorMessage  string                          // Global error message if any artifact is not found
}

// PreCheckAndLoadArtifacts creates a BeforeAgentCallback that loads multiple artifacts into state.
// When StopOnFailure is false, it attempts to load all artifacts and reports all missing ones.
func PreCheckAndLoadArtifacts(cfg PreCheckAndLoadArtifactConfigMulti) agent.BeforeAgentCallback {
	return func(ctx agent.CallbackContext) (*genai.Content, error) {
		state := ctx.State()
		if state == nil {
			return nil, fmt.Errorf("state is nil")
		}

		artService := ctx.Artifacts()
		if artService == nil {
			return nil, fmt.Errorf("artifact service is nil")
		}

		var missingArtifacts []string
		var loadedCount int

		for _, artifactCfg := range cfg.Artifacts {
			if artifactCfg.ErrorMessage == "" {
				artifactCfg.ErrorMessage = fmt.Sprintf("⚠️ Required artifact %q not found.", artifactCfg.ArtifactName)
			}

			// Check if already in state
			val, err := state.Get(artifactCfg.StateKey)
			if err == nil && val != nil {
				observability.Log.Infof("✅ [PreCheck] %s found in state (skipping load)", artifactCfg.StateKey)
				continue
			}

			// Try to load from artifacts
			resp, err := artService.Load(context.Background(), artifactCfg.ArtifactName)
			if err != nil {
				observability.Log.Warnf("⚠️ [PreCheck] Failed to load %s: %v", artifactCfg.ArtifactName, err)
				missingArtifacts = append(missingArtifacts, artifactCfg.ArtifactName)
				if cfg.StopOnFailure {
					return &genai.Content{
						Parts: []*genai.Part{{Text: artifactCfg.ErrorMessage}},
					}, nil
				}
				continue
			}

			if resp == nil || resp.Part == nil || resp.Part.InlineData == nil {
				observability.Log.Warnf("⚠️ [PreCheck] %s is empty or not found", artifactCfg.ArtifactName)
				missingArtifacts = append(missingArtifacts, artifactCfg.ArtifactName)
				if cfg.StopOnFailure {
					return &genai.Content{
						Parts: []*genai.Part{{Text: artifactCfg.ErrorMessage}},
					}, nil
				}
				continue
			}

			// Load into state
			content := string(resp.Part.InlineData.Data)
			if err := state.Set(artifactCfg.StateKey, content); err != nil {
				return nil, fmt.Errorf("failed to set %s in state: %w", artifactCfg.StateKey, err)
			}

			observability.Log.Infof("✅ [PreCheck] Loaded %s into state (%d bytes)", artifactCfg.ArtifactName, len(content))
			loadedCount++
		}

		if len(missingArtifacts) > 0 {
			if cfg.ErrorMessage != "" {
				return &genai.Content{
					Parts: []*genai.Part{{Text: cfg.ErrorMessage}},
				}, nil
			}
			errorMsg := fmt.Sprintf("⚠️ The following required artifacts were not found: %v", missingArtifacts)
			return &genai.Content{
				Parts: []*genai.Part{{Text: errorMsg}},
			}, nil
		}

		observability.Log.Infof("✅ [PreCheck] Loaded %d/%d artifacts successfully", loadedCount, len(cfg.Artifacts))
		return nil, nil
	}
}
