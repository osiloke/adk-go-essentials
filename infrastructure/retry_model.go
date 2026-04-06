package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/osiloke/adk-go-essentials/observability"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// RetryModel wraps a model.LLM and adds retry logic for transient errors.
type RetryModel struct {
	inner      model.LLM
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewRetryModel creates a new RetryModel.
func NewRetryModel(inner model.LLM, maxRetries int, baseDelay time.Duration, maxDelay time.Duration) model.LLM {
	return &RetryModel{
		inner:      inner,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
	}
}

func (r *RetryModel) Name() string {
	return r.inner.Name()
}

func (r *RetryModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		var lastErr error
		for attempt := 0; attempt <= r.maxRetries; attempt++ {
			if attempt > 0 {
				// Exponential backoff: baseDelay * 2^(attempt-1)
				backoff := r.baseDelay * time.Duration(1<<uint(attempt-1))

				// Apply jitter (randomize between 0% and 50% of the backoff)
				// rand.Float64() returns [0.0, 1.0)
				jitter := time.Duration(float64(backoff) * rand.Float64() * 0.5)
				delay := backoff + jitter

				if lastErr != nil {
					errStr := strings.ToLower(lastErr.Error())
					if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) || strings.Contains(errStr, "context canceled") {
						// Context is already dead, no point in retrying
						observability.Log.Warnf("⚠️  [RetryModel] Context canceled or deadline exceeded. Aborting retries.")
						yield(nil, lastErr)
						return
					}
					if strings.Contains(errStr, "maximum context length") || strings.Contains(errStr, "context length") || strings.Contains(errStr, "token limit") {
						// Context length errors are not transient, no point in retrying
						observability.Log.Warnf("⚠️  [RetryModel] Context length error detected. Aborting retries.")
						yield(nil, lastErr)
						return
					}
					if strings.Contains(errStr, "429") || strings.Contains(errStr, "too many requests") || strings.Contains(errStr, "rate limit") {
						observability.Log.Warnf("⚠️  [RetryModel] Rate limit exceeded. Applying extended backoff.")
						delay = delay * 2
					}
				}

				// Cap at maxDelay
				if r.maxDelay > 0 && delay > r.maxDelay {
					delay = r.maxDelay
				}

				if lastErr != nil {
					observability.Log.Infof("🔄 [RetryModel] Attempt %d/%d failed with error: %v. Retrying in %v...", attempt, r.maxRetries, lastErr, delay)
				} else {
					observability.Log.Infof("🔄 [RetryModel] Attempt %d/%d failed. Retrying in %v...", attempt, r.maxRetries, delay)
				}

				select {
				case <-ctx.Done():
					yield(nil, ctx.Err())
					return
				case <-time.After(delay):
				}
			}

			// Create a timeout context for this specific attempt (5 minutes per attempt)
			attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)

			// Call the inner model
			// Since GenerateContent returns an iterator, we need to iterate it to check for errors.
			// If it yields an error immediately, we retry.
			// If it yields a response, we yield it and return (assuming success stream).
			// Note: Streaming retries are tricky. If the stream fails halfway, we can't easily "retry" from the start without re-sending everything.
			// For simplicity, we assume that if the first yield is an error, we retry.
			// If we successfully yielded some data, we assume success and bubble up subsequent errors (or implement more complex resume logic).

			success := true
			observability.Log.Infof("📡 [RetryModel] Calling inner model (attempt %d/%d)...", attempt+1, r.maxRetries+1)
			iter := r.inner.GenerateContent(attemptCtx, req, stream)

			hasYielded := false
			for resp, err := range iter {
				if err != nil {
					lastErr = err
					success = false
					observability.Log.Errorf("❌ [RetryModel] Inner model returned error: %v", err)
					// If we already yielded parts of the stream, retrying from the start might duplicate data for the client.
					// For now, we still break to retry, but ideally we'd want resume logic if hasYielded is true.
					break
				}

				hasYielded = true
				if !yield(resp, nil) {
					cancel()
					return
				}
			}
			cancel()

			if success {
				observability.Log.Infof("✅ [RetryModel] Inner model call completed successfully")
				return
			}

			// If we failed after yielding data, we might not want to transparently retry the whole stream,
			// but to be safe and satisfy the current simple retry model, we just proceed to next attempt.
			if hasYielded {
				observability.Log.Warnf("⚠️  [RetryModel] Note: Stream failed after some data was yielded. Retrying will restart stream.")
			}

			// If we are here, it means we encountered an error in the loop (lastErr is set).
			// We continue to the next attempt.
		}

		// If we exhausted retries, yield the last error
		if lastErr != nil {
			observability.Log.Errorf("❌ [RetryModel] All %d attempts failed. Last error: %v", r.maxRetries, lastErr)

			// Return a JSON error response so the agent can handle it gracefully
			errorJson := fmt.Sprintf(`{"status": "ERROR", "reason": "Model call failed after retries: %v"}`, lastErr)
			observability.Log.Warnf("⚠️  [RetryModel] Returning fallback error JSON: %s", errorJson)

			yield(&model.LLMResponse{
				Content: &genai.Content{
					Parts: []*genai.Part{{Text: errorJson}},
				},
			}, nil)
		}
	}
}
