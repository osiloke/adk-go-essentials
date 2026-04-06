package observability

import (
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/genai"
)

// SaveOutputAsArtifact returns an AfterAgentCallback that saves a value from the session state as an artifact.
func SaveOutputAsArtifact(filename string, stateKey string) agent.AfterAgentCallback {
	return func(ctx agent.CallbackContext) (*genai.Content, error) {
		// Recover from any panics to prevent crashing the server
		defer func() {
			if r := recover(); r != nil {
				Log.Errorf("⚠️  [RECOVER] SaveOutputAsArtifact recovered from panic: %v", r)
			}
		}()

		Log.Infof("📦 [ARTIFACT] Attempting to save %s from key %s...", filename, stateKey)

		if ctx == nil {
			Log.Warnf("⚠️  [ARTIFACT] Context is nil")
			return nil, nil
		}

		state := ctx.State()
		if state == nil {
			Log.Warnf("⚠️  [ARTIFACT] State is nil")
			return nil, nil
		}

		val, err := state.Get(stateKey)
		if err != nil {
			Log.Warnf("⚠️  [ARTIFACT] Error getting key %s: %v", stateKey, err)
			return nil, nil
		}
		if val == nil {
			Log.Warnf("⚠️  [ARTIFACT] Value for key %s is nil", stateKey)
			return nil, nil
		}

		strVal := fmt.Sprintf("%v", val)
		if strings.TrimSpace(strVal) == "" {
			Log.Warnf("⚠️  [ARTIFACT] Value for key %s is empty string", stateKey)
			return nil, nil
		}

		// Determine MIME type based on filename extension
		mimeType := "text/plain"
		if strings.HasSuffix(filename, ".md") {
			mimeType = "text/markdown"
		} else if strings.HasSuffix(filename, ".json") {
			mimeType = "application/json"
		}

		// Create artifact Part
		part := &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: mimeType,
				Data:     []byte(strVal),
			},
		}

		artifacts := ctx.Artifacts()
		if artifacts == nil {
			Log.Warnf("⚠️  [ARTIFACT] Artifacts service is nil")
			return nil, nil
		}

		// Save as artifact using a context that preserves values (session/trace)
		// from the callback context but ignores cancellation, ensuring the save
		// completes even if the callback context is canceled.
		// saveCtx := context.WithoutCancel(ctx)
		_, err = artifacts.Save(ctx, filename, part)
		if err != nil {
			Log.Errorf("⚠️  [ERROR] Failed to save artifact %s: %v", filename, err)
			return nil, nil
		}

		Log.Infof("📦 [ARTIFACT] Successfully saved %s from state key %s (%d bytes)", filename, stateKey, len(strVal))
		return nil, nil
	}
}
