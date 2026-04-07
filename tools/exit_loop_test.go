package tools

import (
	"testing"
)

func TestNewExitLoopTool_CreatesTool(t *testing.T) {
	tool, err := NewExitLoopTool()
	if err != nil {
		t.Fatalf("NewExitLoopTool() error = %v", err)
	}
	if tool == nil {
		t.Fatal("NewExitLoopTool() returned nil tool")
	}
}

func TestExitLoopArgs_ZeroValue(t *testing.T) {
	args := ExitLoopArgs{}
	// Verify the struct is zero-value usable
	var _ ExitLoopArgs = args
}

func TestExitLoopResults_ZeroValue(t *testing.T) {
	results := ExitLoopResults{}
	// Verify the struct is zero-value usable
	var _ ExitLoopResults = results
}
