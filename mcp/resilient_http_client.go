// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"github.com/osiloke/adk-go-essentials/observability"
	"strings"
	"time"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 30 * time.Second

// DefaultRetryMaxAttempts is the default maximum number of retry attempts.
const DefaultRetryMaxAttempts = 3

// DefaultRetryInitialDelay is the initial delay between retries.
const DefaultRetryInitialDelay = 100 * time.Millisecond

// DefaultRetryMaxDelay is the maximum delay between retries.
const DefaultRetryMaxDelay = 10 * time.Second

// HTTPClientConfig configures the resilient HTTP client.
type HTTPClientConfig struct {
	// Timeout is the maximum duration for an HTTP request.
	Timeout time.Duration
	// RetryMaxAttempts is the maximum number of retry attempts.
	RetryMaxAttempts int
	// RetryInitialDelay is the initial delay between retries.
	RetryInitialDelay time.Duration
	// RetryMaxDelay is the maximum delay between retries.
	RetryMaxDelay time.Duration
	// CircuitBreakerMaxFailures is the number of failures before opening the circuit.
	CircuitBreakerMaxFailures int
	// CircuitBreakerResetTimeout is the duration before attempting to close the circuit.
	CircuitBreakerResetTimeout time.Duration
}

// DefaultHTTPClientConfig returns a default HTTP client configuration.
func DefaultHTTPClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		Timeout:                    DefaultHTTPTimeout,
		RetryMaxAttempts:           DefaultRetryMaxAttempts,
		RetryInitialDelay:          DefaultRetryInitialDelay,
		RetryMaxDelay:              DefaultRetryMaxDelay,
		CircuitBreakerMaxFailures:  5,
		CircuitBreakerResetTimeout: 30 * time.Second,
	}
}

// isRetryableError determines if an error is retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network-related errors that are typically transient
	if strings.Contains(errStr, "TLS handshake timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "temporary failure in address resolution") {
		return true
	}

	// Check for net.OpError and net.DNSError
	var netErr net.Error
	if As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}

	return false
}

// As is a helper to mimic errors.As for our use case.
func As(err error, target interface{}) bool {
	// Simple implementation - in production, use errors.As from stdlib
	targetPtr, ok := target.(*net.Error)
	if !ok {
		return false
	}

	for e := err; e != nil; {
		if netErr, ok := e.(net.Error); ok {
			*targetPtr = netErr
			return true
		}
		// Unwrap if possible
		type unwrapper interface {
			Unwrap() error
		}
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

// retryableTransport wraps an http.RoundTripper with retry logic.
type retryableTransport struct {
	base           http.RoundTripper
	config         *HTTPClientConfig
	circuitBreaker *circuitBreaker
}

// newRetryableTransport creates a new retryable transport.
func newRetryableTransport(config *HTTPClientConfig) *retryableTransport {
	return &retryableTransport{
		base:   http.DefaultTransport,
		config: config,
		circuitBreaker: newCircuitBreaker(
			config.CircuitBreakerMaxFailures,
			config.CircuitBreakerResetTimeout,
		),
	}
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	var lastErr error
	delay := t.config.RetryInitialDelay

	for attempt := 1; attempt <= t.config.RetryMaxAttempts; attempt++ {
		// Check circuit breaker before attempting request
		if !t.circuitBreaker.AllowRequest() {
			observability.Log.Debugf("[MCP HTTP] Circuit breaker open, rejecting request to %s", req.URL.Host)
			return nil, fmt.Errorf("circuit breaker open: too many failures")
		}

		resp, err := t.base.RoundTrip(req)

		if err == nil {
			// Success - reset circuit breaker
			t.circuitBreaker.RecordSuccess()
			return resp, nil
		}

		lastErr = err
		observability.Log.Debugf("[MCP HTTP] Request failed (attempt %d/%d): %v", attempt, t.config.RetryMaxAttempts, err)

		// Record failure in circuit breaker
		t.circuitBreaker.RecordFailure()

		// Check if error is retryable
		if !isRetryableError(err) {
			observability.Log.Debugf("[MCP HTTP] Non-retryable error: %v", err)
			return nil, err
		}

		// Don't retry on last attempt
		if attempt == t.config.RetryMaxAttempts {
			observability.Log.Debugf("[MCP HTTP] Max retries reached, giving up")
			return nil, err
		}

		// Wait before retrying (exponential backoff)
		select {
		case <-ctx.Done():
			observability.Log.Debugf("[MCP HTTP] Context cancelled during retry wait")
			return nil, ctx.Err()
		case <-time.After(delay):
			// Exponential backoff with max delay cap
			delay *= 2
			if delay > t.config.RetryMaxDelay {
				delay = t.config.RetryMaxDelay
			}
		}
	}

	return nil, lastErr
}

// circuitBreaker implements a simple circuit breaker pattern.
type circuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failures     int
	lastFailure  time.Time
	state        string // "closed", "open", "half-open"
}

// newCircuitBreaker creates a new circuit breaker.
func newCircuitBreaker(maxFailures int, resetTimeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        "closed",
	}
}

// AllowRequest returns true if a request should be attempted.
func (cb *circuitBreaker) AllowRequest() bool {
	switch cb.state {
	case "closed":
		return true
	case "open":
		// Check if enough time has passed to try again
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			observability.Log.Debugf("[MCP CircuitBreaker] Transitioning from open to half-open")
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open":
		// Allow one request to test if service is back
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request.
func (cb *circuitBreaker) RecordSuccess() {
	if cb.state == "half-open" {
		observability.Log.Debugf("[MCP CircuitBreaker] Transitioning from half-open to closed")
		cb.state = "closed"
	}
	cb.failures = 0
}

// RecordFailure records a failed request.
func (cb *circuitBreaker) RecordFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.maxFailures && cb.state == "closed" {
		observability.Log.Debugf("[MCP CircuitBreaker] Transitioning from closed to open (%d failures)", cb.failures)
		cb.state = "open"
	}
}

// apiKeyTransport is a custom http.RoundTripper that adds API key authentication with retry logic.
type apiKeyTransport struct {
	apiKey   string
	endpoint string
	base     http.RoundTripper
	config   *HTTPClientConfig
}

// newAPIKeyTransport creates a new API key transport with retry logic.
func newAPIKeyTransport(apiKey, endpoint string, config *HTTPClientConfig) *apiKeyTransport {
	return &apiKeyTransport{
		apiKey:   apiKey,
		endpoint: endpoint,
		base:     newRetryableTransport(config),
		config:   config,
	}
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())

	// Add API key as Authorization header (Bearer token format)
	// For Google APIs, also try X-Goog-API-Key header
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Goog-API-Key", t.apiKey)

	// Log ALL requests for debugging
	observability.Log.Infof("[MCP HTTP] --> REQUEST: %s %s (from: %s)", req.Method, req.URL.String(), req.Header.Get("Referer"))

	// Log request details for list_projects
	if strings.Contains(req.URL.Path, "list_projects") || strings.Contains(req.URL.String(), "list_projects") {
		observability.Log.Infof("[MCP HTTP] --> list_projects request: %s %s", req.Method, req.URL.String())
	}

	resp, err := t.base.RoundTrip(req)

	// Handle error - note: resp will be nil on error
	if err != nil {
		observability.Log.Debugf("[MCP HTTP] Request failed: %v", err)
		// Only log response details if response is not nil
		return nil, err
	}

	// Log response status for successful requests
	observability.Log.Debugf("[MCP HTTP] <-- Response Status: %d %s", resp.StatusCode, resp.Status)

	// Log response body for list_projects
	if strings.Contains(req.URL.Path, "list_projects") || strings.Contains(req.URL.String(), "list_projects") {
		if resp.Body != nil {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				observability.Log.Warnf("[MCP HTTP] Failed to read response body: %v", readErr)
			} else {
				observability.Log.Infof("[MCP HTTP] <-- list_projects response body: %s", string(body))
				// Restore body for further reading
				resp.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
	}

	return resp, nil
}

// CreateHTTPClient creates a resilient HTTP client with the given configuration.
func CreateHTTPClient(config *HTTPClientConfig) *http.Client {
	return &http.Client{
		Timeout:   config.Timeout,
		Transport: newRetryableTransport(config),
	}
}
