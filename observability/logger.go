package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Logger defines the standard logging interface for the project,
// gracefully bridging formatted unstructured logs and structured key-value arrays.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)

	With(args ...any) Logger
	WithContext(ctx context.Context) Logger
}

// slogLogger is the default implementation of Logger backed by log/slog.
type slogLogger struct {
	logger *slog.Logger
}

func (s *slogLogger) Debug(msg string, args ...any) { s.logger.Debug(msg, args...) }
func (s *slogLogger) Info(msg string, args ...any)  { s.logger.Info(msg, args...) }
func (s *slogLogger) Warn(msg string, args ...any)  { s.logger.Warn(msg, args...) }
func (s *slogLogger) Error(msg string, args ...any) { s.logger.Error(msg, args...) }

func (s *slogLogger) Debugf(format string, args ...any) {
	s.logger.Debug(fmt.Sprintf(format, args...))
}
func (s *slogLogger) Infof(format string, args ...any) {
	s.logger.Info(fmt.Sprintf(format, args...))
}
func (s *slogLogger) Warnf(format string, args ...any) {
	s.logger.Warn(fmt.Sprintf(format, args...))
}
func (s *slogLogger) Errorf(format string, args ...any) {
	s.logger.Error(fmt.Sprintf(format, args...))
}

func (s *slogLogger) With(args ...any) Logger {
	return &slogLogger{logger: s.logger.With(args...)}
}

func (s *slogLogger) WithContext(ctx context.Context) Logger {
	// If you have context extractors (like for trace IDs), you'd add them here.
	// For now, it just returns the same logger, as slog.Logger doesn't directly
	// enrich itself from context without custom handlers.
	return s
}

// Log is the global default logger for the project.
var Log Logger

func init() {
	// Initialize with a default text handler capable of showing debug logs
	// depending on environment variables in the future.
	// We use an explicit TextHandler to ensure consistent output formats.
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Or read from env: os.Getenv("LOG_LEVEL")
	})
	Log = &slogLogger{logger: slog.New(handler)}
}

// LogStart logs the beginning of an agent's execution, including a summary of its state.
func LogStart(ctx agent.CallbackContext) (*genai.Content, error) {
	Log.Info("🚦 [START] Agent", "name", ctx.AgentName(), "branch", ctx.Branch())

	// Log the input message
	Log.Info("[LogStart] Agent invoked", "name", ctx.AgentName())

	// Inspect State Keys using iterator
	state := ctx.State()
	if state == nil {
		Log.Debug("   📝 State: <nil>")
		return nil, nil
	}

	var keysInfo []string
	for k, v := range state.All() {
		keysInfo = append(keysInfo, fmt.Sprintf("%s (len=%d)", k, len(fmt.Sprintf("%v", v))))

		if k == "autocot_result" {
			strVal := fmt.Sprintf("%v", v)
			if len(strVal) > 500 {
				Log.Debug("   🔍 autocot_result (preview)", "content", strVal[:500]+"...")
			} else {
				Log.Debug("   🔍 autocot_result", "content", strVal)
			}
		}
	}
	Log.Debug("   📝 State Keys", "keys", keysInfo)

	Log.Debug("[LogStart] Returning nil, nil - allowing agent to proceed")
	return nil, nil
}

// LogEnd logs the end of an agent's execution.
func LogEnd(ctx agent.CallbackContext) (*genai.Content, error) {
	Log.Info("✅ [END] Agent", "name", ctx.AgentName())
	return nil, nil
}

// LogLLMRequest logs the beginning of an LLM request, including system instruction size.
// This is a llmagent.BeforeModelCallback that logs when the model is about to be called.
func LogLLMRequest(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req.Config.SystemInstruction != nil && len(req.Config.SystemInstruction.Parts) > 0 {
		totalChars := 0
		for _, part := range req.Config.SystemInstruction.Parts {
			totalChars += len(part.Text)
		}
		Log.Infof("🧠 [LLM] Request started: %d system instruction parts, %d characters",
			len(req.Config.SystemInstruction.Parts), totalChars)
	} else {
		Log.Infof("🧠 [LLM] Request started (no system instruction)")
	}
	return nil, nil
}

// LogLLMResponse logs the model's response, including tool calls or text response.
// This is a llmagent.AfterModelCallback that logs the LLM's output.
func LogLLMResponse(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	if err != nil {
		Log.Errorf("❌ [LLM] Response error: %v", err)
		return resp, err
	}
	if resp == nil {
		Log.Warnf("⚠️  [LLM] Returned nil response")
		return resp, nil
	}
	if resp.Content == nil || len(resp.Content.Parts) == 0 {
		Log.Warnf("⚠️  [LLM] Returned empty content")
		return resp, nil
	}

	// Log tool calls
	var toolNames []string
	for _, part := range resp.Content.Parts {
		if part.FunctionCall != nil {
			toolNames = append(toolNames, part.FunctionCall.Name)
		}
	}
	if len(toolNames) > 0 {
		Log.Infof("🔧 [LLM] Called tools: [%s]", strings.Join(toolNames, ", "))
	} else {
		Log.Infof("💬 [LLM] Responded with text")
	}
	return resp, nil
}

// LogLLMRequestVerbose is a verbose version that logs additional request details.
// Use this for debugging complex agent interactions.
func LogLLMRequestVerbose(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req.Config.SystemInstruction != nil && len(req.Config.SystemInstruction.Parts) > 0 {
		totalChars := 0
		for _, part := range req.Config.SystemInstruction.Parts {
			totalChars += len(part.Text)
		}
		Log.Infof("🧠 [LLM] Request started: %d system instruction parts, %d characters",
			len(req.Config.SystemInstruction.Parts), totalChars)
		if len(req.Config.SystemInstruction.Parts) > 0 {
			Log.Debugf("📝 [LLM] System instruction preview: %.500s...", req.Config.SystemInstruction.Parts[0].Text)
		}
	} else {
		Log.Infof("🧠 [LLM] Request started (no system instruction)")
	}

	return nil, nil
}

// LogLLMResponseVerbose is a verbose version that logs full response details.
// Use this for debugging complex agent interactions.
func LogLLMResponseVerbose(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	if err != nil {
		Log.Errorf("❌ [LLM] Response error: %v", err)
		return resp, err
	}
	if resp == nil {
		Log.Warnf("⚠️  [LLM] Returned nil response")
		return resp, nil
	}
	if resp.Content == nil || len(resp.Content.Parts) == 0 {
		Log.Warnf("⚠️  [LLM] Returned empty content")
		return resp, nil
	}

	// Log tool calls with details
	var toolNames []string
	for _, part := range resp.Content.Parts {
		if part.FunctionCall != nil {
			toolNames = append(toolNames, part.FunctionCall.Name)
			Log.Debugf("🔧 [LLM] Tool call: %s with %d args",
				part.FunctionCall.Name, len(part.FunctionCall.Args))
		}
	}
	if len(toolNames) > 0 {
		Log.Infof("🔧 [LLM] Called tools: [%s]", strings.Join(toolNames, ", "))
	} else {
		// Log text response preview
		for _, part := range resp.Content.Parts {
			if part.Text != "" {
				if len(part.Text) > 500 {
					Log.Infof("💬 [LLM] Text response: %.500s...", part.Text[:500])
				} else {
					Log.Infof("💬 [LLM] Text response: %s", part.Text)
				}
			}
		}
	}
	return resp, nil
}
