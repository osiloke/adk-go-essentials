package state

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

var (
	// CommonKeys always injected by InjectSpecificKeys if available.
	CommonKeys = []string{"autocot_result", "execution_plan"}
	// DefaultKeys injected by InjectState. Customize as needed for your application.
	DefaultKeys = append([]string{}, CommonKeys...)

	// markdownJSONRegex matches JSON blocks within markdown.
	markdownJSONRegex = regexp.MustCompile(`(?s)` + "```" + `(?:json)?\n(.*?)\n` + "```")
)

// MapAutoCoTResult extracts the execution plan from the autocot_result and puts it at the root
// of the session state so subsequent agents can easily access it via placeholders.
func MapAutoCoTResult(ctx agent.CallbackContext) (*genai.Content, error) {
	state := ctx.State()
	if state == nil {
		return nil, nil
	}
	val, err := state.Get("autocot_result")
	if err != nil {
		return nil, nil // Not found, maybe it's the first run
	}
	if val == nil {
		return nil, nil
	}

	resultMap, err := ParseAutoCoTResult(val)
	if err != nil {
		slog.Warn("Failed to parse autocot_result", "error", err)
		return nil, nil
	}

	if plan, ok := resultMap["execution_plan"]; ok {
		if err := state.Set("execution_plan", plan); err != nil {
			slog.Warn("Failed to set execution_plan in state", "error", err)
		}
	}

	return nil, nil
}

// ParseAutoCoTResult cleans and parses the autocot_result from state.
// It handles both raw JSON strings (including those wrapped in markdown code blocks)
// and map[string]any types.
//
// If the value is a string that contains no parseable JSON object, it does NOT return
// an error. Instead, it returns a graceful NEEDS_INFO fallback map that includes the
// raw model text as the reasoning_trace. This allows downstream agents to handle
// unstructured output gracefully rather than crashing.
func ParseAutoCoTResult(val any) (map[string]any, error) {
	if val == nil {
		return nil, fmt.Errorf("autocot_result is nil")
	}

	var resultMap map[string]any
	if s, ok := val.(string); ok {
		cleaned := s

		// Extract JSON from Markdown block if present
		matches := markdownJSONRegex.FindStringSubmatch(cleaned)
		if len(matches) > 1 {
			cleaned = matches[1]
		} else {
			// Fallback: Find first '{' and last '}' to handle potential conversational noise
			start := strings.Index(cleaned, "{")
			end := strings.LastIndex(cleaned, "}")
			if start != -1 && end != -1 && end > start {
				cleaned = cleaned[start : end+1]
			}
		}

		if err := json.Unmarshal([]byte(cleaned), &resultMap); err != nil {
			// The model output prose instead of JSON. Degrade gracefully to NEEDS_INFO
			// so downstream agents can handle it rather than crashing.
			slog.Warn("AutoCoT returned non-JSON output, degrading to NEEDS_INFO fallback", "raw", s)
			return map[string]any{
				"status":          "NEEDS_INFO",
				"reasoning_trace": s,
				"missing_info":    []any{"The planning step did not return structured output. Could you please clarify your request?"},
			}, nil
		}
	} else if m, ok := val.(map[string]any); ok {
		resultMap = m
	} else {
		return nil, fmt.Errorf("unexpected type for autocot_result: %T", val)
	}

	return resultMap, nil
}

// InjectState is a BeforeModelCallback that injects relevant session state into the LLM prompt.
// It injects the DefaultKeys. Use InjectSpecificKeys for custom injection.
func InjectState(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	return injectKeysInternal(ctx, req, DefaultKeys)
}

// InjectSpecificKeys creates a callback that injects the specified keys from state,
// in addition to the CommonKeys.
func InjectSpecificKeys(keys ...string) llmagent.BeforeModelCallback {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		// Always include common context if available
		allKeys := append([]string{}, CommonKeys...)
		allKeys = append(allKeys, keys...)
		return injectKeysInternal(ctx, req, allKeys)
	}
}

func injectKeysInternal(ctx agent.CallbackContext, req *model.LLMRequest, keys []string) (*model.LLMResponse, error) {
	state := ctx.State()
	if state == nil {
		return nil, nil
	}

	var contextLines []string
	seen := make(map[string]bool)

	for _, k := range keys {
		if seen[k] {
			continue
		}
		seen[k] = true
		val, err := state.Get(k)
		if err == nil && val != nil {
			contextLines = append(contextLines, fmt.Sprintf("---\n%s\n---\n%v\n", k, val))
		}
	}

	if len(contextLines) == 0 {
		return nil, nil
	}

	injection := "\n\n### PROJECT CONTEXT FROM STATE\n" + strings.Join(contextLines, "\n")

	if req.Config.SystemInstruction == nil {
		req.Config.SystemInstruction = &genai.Content{
			Role:  "system",
			Parts: []*genai.Part{{Text: injection}},
		}
	} else {
		if len(req.Config.SystemInstruction.Parts) > 0 {
			req.Config.SystemInstruction.Parts[0].Text += injection
		} else {
			req.Config.SystemInstruction.Parts = append(req.Config.SystemInstruction.Parts, &genai.Part{Text: injection})
		}
	}

	return nil, nil
}
