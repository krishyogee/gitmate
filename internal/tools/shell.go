package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/krishyogee/gitmate/internal/approval"
)

type RunTestsTool struct {
	Command string
}

func (t RunTestsTool) Name() string                  { return "run_tests" }
func (t RunTestsTool) Description() string           { return "Run configured test command" }
func (t RunTestsTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t RunTestsTool) Execute(ctx context.Context, _ string) (string, error) {
	return runShell(ctx, t.Command)
}

type RunLintTool struct {
	Command string
}

func (t RunLintTool) Name() string                  { return "run_lint" }
func (t RunLintTool) Description() string           { return "Run configured lint command" }
func (t RunLintTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t RunLintTool) Execute(ctx context.Context, _ string) (string, error) {
	return runShell(ctx, t.Command)
}

func runShell(ctx context.Context, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("no command configured")
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	combined := stdout.String() + stderr.String()
	if err != nil {
		return combined, fmt.Errorf("%s: %w", command, err)
	}
	return combined, nil
}
