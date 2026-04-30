package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/krishyogee/gitmate/internal/approval"
)

type CreatePRTool struct {
	Base string
}

type PRInput struct {
	Title string
	Body  string
	Base  string
	Draft bool
}

func (t CreatePRTool) Name() string                  { return "create_pr" }
func (t CreatePRTool) Description() string           { return "Create a draft PR via gh CLI" }
func (t CreatePRTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t CreatePRTool) Execute(ctx context.Context, input string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI not installed; install from https://cli.github.com")
	}

	parts := strings.SplitN(input, "\n\n", 2)
	title := strings.TrimSpace(parts[0])
	body := ""
	if len(parts) == 2 {
		body = parts[1]
	}
	if title == "" {
		return "", fmt.Errorf("PR title required")
	}
	base := t.Base
	if base == "" {
		base = "main"
	}

	args := []string{"pr", "create", "--title", title, "--body", body, "--base", base, "--draft"}
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stderr.String(), fmt.Errorf("gh pr create: %w", err)
	}
	return stdout.String(), nil
}

func PushBranch(ctx context.Context) (string, error) {
	branch, err := CurrentBranch(ctx)
	if err != nil {
		return "", err
	}
	out, err := runGit(ctx, "push", "-u", "origin", branch)
	if err != nil {
		return out, err
	}
	return out, nil
}
