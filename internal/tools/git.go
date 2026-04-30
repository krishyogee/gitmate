package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/approval"
)

func RunGit(ctx context.Context, args ...string) (string, error) {
	return runGit(ctx, args...)
}

func runGit(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func RepoRoot(ctx context.Context) (string, error) {
	out, err := runGit(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func CurrentBranch(ctx context.Context) (string, error) {
	out, err := runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

type GitDiffTool struct{}

func (t GitDiffTool) Name() string                   { return "git_diff" }
func (t GitDiffTool) Description() string            { return "Get staged diff (compressed if large)" }
func (t GitDiffTool) RiskLevel() approval.RiskLevel  { return approval.READ }
func (t GitDiffTool) Execute(ctx context.Context, input string) (string, error) {
	args := []string{"diff", "--staged"}
	if strings.TrimSpace(input) == "unstaged" {
		args = []string{"diff"}
	}
	out, err := runGit(ctx, args...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		un, _ := runGit(ctx, "diff")
		if strings.TrimSpace(un) != "" {
			return ai.SummarizeDiff(un), nil
		}
		return "(no changes)", nil
	}
	return ai.SummarizeDiff(out), nil
}

type GitStatusTool struct{}

func (t GitStatusTool) Name() string                  { return "git_status" }
func (t GitStatusTool) Description() string           { return "Short repo status" }
func (t GitStatusTool) RiskLevel() approval.RiskLevel { return approval.READ }
func (t GitStatusTool) Execute(ctx context.Context, _ string) (string, error) {
	return runGit(ctx, "status", "--short", "--branch")
}

type GitLogTool struct{}

func (t GitLogTool) Name() string                  { return "git_log" }
func (t GitLogTool) Description() string           { return "Recent commits (oneline, last 20)" }
func (t GitLogTool) RiskLevel() approval.RiskLevel { return approval.READ }
func (t GitLogTool) Execute(ctx context.Context, input string) (string, error) {
	n := "20"
	if input != "" {
		n = strings.TrimSpace(input)
	}
	return runGit(ctx, "log", "--oneline", "-n", n)
}

type GitCommitTool struct{}

func (t GitCommitTool) Name() string                  { return "git_commit" }
func (t GitCommitTool) Description() string           { return "Commit staged changes with message" }
func (t GitCommitTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t GitCommitTool) Execute(ctx context.Context, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("empty commit message")
	}
	return runGit(ctx, "commit", "-m", message)
}

type GitFetchTool struct{}

func (t GitFetchTool) Name() string                  { return "git_fetch" }
func (t GitFetchTool) Description() string           { return "Fetch remote refs" }
func (t GitFetchTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t GitFetchTool) Execute(ctx context.Context, _ string) (string, error) {
	return runGit(ctx, "fetch", "--all", "--prune")
}

type GitRebaseTool struct {
	Base string
}

func (t GitRebaseTool) Name() string                  { return "git_rebase" }
func (t GitRebaseTool) Description() string           { return "Rebase onto base branch" }
func (t GitRebaseTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t GitRebaseTool) Execute(ctx context.Context, input string) (string, error) {
	base := t.Base
	if strings.TrimSpace(input) != "" {
		base = strings.TrimSpace(input)
	}
	if base == "" {
		base = "main"
	}
	return runGit(ctx, "rebase", base)
}

type GitMergeTool struct {
	Base string
}

func (t GitMergeTool) Name() string                  { return "git_merge" }
func (t GitMergeTool) Description() string           { return "Merge base into current branch" }
func (t GitMergeTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t GitMergeTool) Execute(ctx context.Context, input string) (string, error) {
	base := t.Base
	if strings.TrimSpace(input) != "" {
		base = strings.TrimSpace(input)
	}
	if base == "" {
		base = "main"
	}
	return runGit(ctx, "merge", "--no-ff", base)
}

type GitStashTool struct{}

func (t GitStashTool) Name() string                  { return "git_stash" }
func (t GitStashTool) Description() string           { return "Stash dirty working tree" }
func (t GitStashTool) RiskLevel() approval.RiskLevel { return approval.EXECUTE }
func (t GitStashTool) Execute(ctx context.Context, _ string) (string, error) {
	return runGit(ctx, "stash", "push", "-u", "-m", "gitmate auto-stash")
}

type FetchHotspotsTool struct{}

func (t FetchHotspotsTool) Name() string                  { return "fetch_hotspots" }
func (t FetchHotspotsTool) Description() string           { return "Files with high edit frequency on base" }
func (t FetchHotspotsTool) RiskLevel() approval.RiskLevel { return approval.READ }
func (t FetchHotspotsTool) Execute(ctx context.Context, input string) (string, error) {
	base := strings.TrimSpace(input)
	if base == "" {
		base = "main"
	}
	out, err := runGit(ctx, "log", "--name-only", "--pretty=format:", base, "-n", "200")
	if err != nil {
		return "", err
	}
	counts := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.TrimSpace(line)
		if f == "" {
			continue
		}
		counts[f]++
	}
	type fc struct {
		f string
		c int
	}
	var arr []fc
	for f, c := range counts {
		arr = append(arr, fc{f, c})
	}
	for i := 0; i < len(arr); i++ {
		for j := i + 1; j < len(arr); j++ {
			if arr[j].c > arr[i].c {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}
	limit := 15
	if limit > len(arr) {
		limit = len(arr)
	}
	var b strings.Builder
	for i := 0; i < limit; i++ {
		fmt.Fprintf(&b, "%d %s\n", arr[i].c, arr[i].f)
	}
	return b.String(), nil
}
