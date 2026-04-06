package agent

import (
	"fmt"
	"iter"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Safe wraps an agent to prevent ADK's loopagent from panicking on nil events.
// This is a workaround for a bug in ADK where loopagent attempts to access
// event.Actions without checking if the event is nil (which happens on errors).
func Safe(a agent.Agent) agent.Agent {
	if a == nil {
		return nil
	}
	return &safeAgent{a}
}

// safeAgent is a decorator that intercepts nil events from the underlying agent.
type safeAgent struct {
	agent.Agent
}

// Run implements the agent.Agent interface by delegating to the wrapped agent
// and ensuring all yielded events are non-nil.
func (s *safeAgent) Run(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		for ev, err := range s.Agent.Run(ctx) {
			if ev == nil && err != nil {
				// Create a dummy event to prevent ADK loopagent from panicking
				ev = session.NewEvent(ctx.InvocationID())
				ev.Author = s.Agent.Name()
				ev.Branch = ctx.Branch()
				ev.Content = &genai.Content{
					Parts: []*genai.Part{{Text: fmt.Sprintf("Error occurred in %s: %v", s.Agent.Name(), err)}},
				}
			}
			if !yield(ev, err) {
				return
			}
		}
	}
}
