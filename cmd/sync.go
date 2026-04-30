package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/tools"
)

var syncCmd = &cobra.Command{
	Use:   "sync [base]",
	Short: "Sync current branch with base (default: main) — pause on conflict",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		ctx := context.Background()
		if app.RepoRoot == "" {
			return fmt.Errorf("not in a git repo")
		}
		base := app.Cfg.DefaultBase
		if len(args) == 1 {
			base = args[0]
		}

		status, err := tools.GitStatusTool{}.Execute(ctx, "")
		if err != nil {
			return err
		}
		dirty := isDirty(status)

		if dirty && app.Cfg.AutoStash {
			card := approval.Card{
				Action:      "git_stash",
				Description: "working tree dirty; stash before sync",
			}
			dec, _, err := app.Approval.Request(card)
			if err != nil {
				return err
			}
			if dec == approval.DecisionNo {
				return fmt.Errorf("aborted: dirty working tree")
			}
			if out, err := (tools.GitStashTool{}).Execute(ctx, ""); err != nil {
				fmt.Println(out)
				return err
			}
		}

		fmt.Println("fetching...")
		if out, err := (tools.GitFetchTool{}).Execute(ctx, ""); err != nil {
			fmt.Println(out)
			return err
		}

		var syncTool interface {
			Execute(ctx context.Context, input string) (string, error)
		}
		if app.Cfg.SyncMode == "merge" {
			syncTool = tools.GitMergeTool{Base: base}
		} else {
			syncTool = tools.GitRebaseTool{Base: base}
		}

		out, err := syncTool.Execute(ctx, base)
		fmt.Println(out)
		if err == nil {
			fmt.Println("✓ sync clean")
			return nil
		}

		fmt.Println("\nconflicts detected. Run `gitmate resolve <file>` for each conflicted file, then continue:")
		fmt.Printf("  git %s --continue\n", app.Cfg.SyncMode)
		conflicted, _ := listConflictedFiles(ctx)
		for _, f := range conflicted {
			fmt.Println("  -", f)
		}
		if app.Cfg.Guardrails.AlwaysRunTestsAfterConflict {
			fmt.Println("\nReminder: run tests after resolution (config.alwaysRunTestsAfterConflict=true)")
		}
		return nil
	},
}

func isDirty(status string) bool {
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "##") {
			continue
		}
		return true
	}
	return false
}

func listConflictedFiles(ctx context.Context) ([]string, error) {
	out, err := (tools.GitStatusTool{}).Execute(ctx, "")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		if strings.Contains(code, "U") || code == "AA" || code == "DD" {
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}
