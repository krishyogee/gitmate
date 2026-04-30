package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/conflict"
)

type ParseConflictsTool struct{}

func (t ParseConflictsTool) Name() string                  { return "parse_conflicts" }
func (t ParseConflictsTool) Description() string           { return "Parse <<< === >>> blocks in file" }
func (t ParseConflictsTool) RiskLevel() approval.RiskLevel { return approval.READ }
func (t ParseConflictsTool) Execute(ctx context.Context, file string) (string, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return "", fmt.Errorf("file path required")
	}
	blocks, err := conflict.ParseFile(file)
	if err != nil {
		return "", err
	}
	if len(blocks) == 0 {
		return "(no conflicts found)", nil
	}
	var b strings.Builder
	for i, blk := range blocks {
		c := conflict.Classify(blk)
		fmt.Fprintf(&b, "Block %d (line %d, complexity=%s):\n--- ours ---\n%s\n--- theirs ---\n%s\n\n",
			i+1, blk.StartLine, c, strings.Join(blk.OursLines, "\n"), strings.Join(blk.TheirsLines, "\n"))
	}
	return b.String(), nil
}

type ResolveConflictTool struct{}

func (t ResolveConflictTool) Name() string                  { return "resolve_conflict" }
func (t ResolveConflictTool) Description() string           { return "Apply resolved content to a file" }
func (t ResolveConflictTool) RiskLevel() approval.RiskLevel { return approval.PROPOSE }
func (t ResolveConflictTool) Execute(ctx context.Context, input string) (string, error) {
	parts := strings.SplitN(input, "\n", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("expected first line=path, rest=content")
	}
	path := strings.TrimSpace(parts[0])
	content := parts[1]
	if strings.Contains(content, "<<<<<<<") || strings.Contains(content, ">>>>>>>") {
		return "", fmt.Errorf("resolution still contains conflict markers")
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	if _, err := runGit(ctx, "add", path); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %s and staged", path), nil
}
