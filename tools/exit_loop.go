package tools

import (
	"github.com/osiloke/adk-go-essentials/observability"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// ExitLoopArgs defines the (empty) arguments for the ExitLoop tool.
type ExitLoopArgs struct{}

// ExitLoopResults defines the output of the ExitLoop tool.
type ExitLoopResults struct{}

// ExitLoop is a tool that signals a loop agent to terminate by setting Escalate to true.
func ExitLoop(ctx tool.Context, input ExitLoopArgs) (ExitLoopResults, error) {
	observability.Log.Infof("🛠️ [Tool Call] exitLoop triggered by %s", ctx.AgentName())
	ctx.Actions().Escalate = true
	return ExitLoopResults{}, nil
}

// NewExitLoopTool creates a tool that allows an agent to signal loop termination.
func NewExitLoopTool() (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "exitLoop",
			Description: "Call this function ONLY when the task is complete and the evaluation passes.",
		},
		ExitLoop,
	)
}
