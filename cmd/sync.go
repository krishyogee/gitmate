package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/tools"
	"github.com/krishyogee/gitmate/internal/tui"
)

var syncCmd = &cobra.Command{
	Use:   "sync [base]",
	Short: "Fetch + integrate origin/<branch> + integrate base — pause on conflict",
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
		branch, err := tools.CurrentBranch(ctx)
		if err != nil {
			return err
		}

		status, err := tools.GitStatusTool{}.Execute(ctx, "")
		if err != nil {
			return err
		}
		if isDirty(status) && app.Cfg.AutoStash {
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

		stream := tui.NewStream()
		stream.Start("fetching all remotes")
		if out, err := (tools.GitFetchTool{}).Execute(ctx, ""); err != nil {
			stream.Fail("fetch failed")
			fmt.Println(out)
			return err
		}
		stream.Done("fetch complete")

		// Step 1: integrate origin/<current-branch> if it exists and is behind
		if remoteRef := "origin/" + branch; remoteExists(ctx, remoteRef) {
			ahead, behind, err := compareRefs(ctx, "HEAD", remoteRef)
			if err != nil {
				return fmt.Errorf("compare origin/%s: %w", branch, err)
			}
			stream.Info(fmt.Sprintf("origin/%s — ahead=%d behind=%d", branch, ahead, behind))
			if behind > 0 {
				if err := integrate(ctx, app, remoteRef, fmt.Sprintf("origin/%s", branch)); err != nil {
					return reportConflicts(ctx, app, err)
				}
				stream.Done(fmt.Sprintf("integrated origin/%s", branch))
			} else {
				stream.Info("up-to-date with own remote")
			}
		} else {
			stream.Info(fmt.Sprintf("no origin/%s yet (will create on first push)", branch))
		}

		// Step 2: integrate base
		if branch == base {
			stream.Info("on base branch — skipping base integration")
			return nil
		}
		if !branchExists(ctx, base) {
			return fmt.Errorf("base branch %q not found locally or on origin (try `git fetch` or `--base <name>`)", base)
		}
		ahead, behind, err := compareRefs(ctx, "HEAD", base)
		if err != nil {
			return fmt.Errorf("compare %s: %w", base, err)
		}
		stream.Info(fmt.Sprintf("%s — ahead=%d behind=%d", base, ahead, behind))
		if behind == 0 {
			stream.Done("already up-to-date with base")
			return nil
		}
		if err := integrate(ctx, app, base, base); err != nil {
			return reportConflicts(ctx, app, err)
		}
		stream.Done(fmt.Sprintf("integrated %s", base))
		return nil
	},
}

func remoteExists(ctx context.Context, ref string) bool {
	_, err := tools.RunGit(ctx, "rev-parse", "--verify", ref)
	return err == nil
}

func branchExists(ctx context.Context, name string) bool {
	if _, err := tools.RunGit(ctx, "rev-parse", "--verify", name); err == nil {
		return true
	}
	if _, err := tools.RunGit(ctx, "rev-parse", "--verify", "origin/"+name); err == nil {
		return true
	}
	return false
}

func compareRefs(ctx context.Context, ref, target string) (ahead, behind int, err error) {
	out, err := tools.RunGit(ctx, "rev-list", "--left-right", "--count", target+"..."+ref)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected: %s", out)
	}
	fmt.Sscanf(parts[0], "%d", &behind)
	fmt.Sscanf(parts[1], "%d", &ahead)
	return ahead, behind, nil
}

func integrate(ctx context.Context, app *App, target, label string) error {
	mode := app.Cfg.SyncMode
	card := approval.Card{
		Action:      "git_" + mode,
		Input:       fmt.Sprintf("git %s %s", mode, target),
		Description: fmt.Sprintf("%s %s into current branch", mode, label),
	}
	dec, _, err := app.Approval.Request(card)
	if err != nil {
		return err
	}
	if dec == approval.DecisionNo {
		return fmt.Errorf("aborted: integrate %s denied", label)
	}
	var args []string
	if mode == "merge" {
		args = []string{"merge", "--no-ff", target}
	} else {
		args = []string{"rebase", target}
	}
	out, err := tools.RunGit(ctx, args...)
	fmt.Println(out)
	return err
}

func reportConflicts(ctx context.Context, app *App, err error) error {
	fmt.Println("\nconflicts detected. Run `gitmate resolve <file>` per file, then continue:")
	fmt.Printf("  git %s --continue\n", app.Cfg.SyncMode)
	conflicted, _ := listConflictedFiles(ctx)
	for _, f := range conflicted {
		fmt.Println("  -", f)
	}
	if app.Cfg.Guardrails.AlwaysRunTestsAfterConflict {
		fmt.Println("\nReminder: run tests after resolution (config.alwaysRunTestsAfterConflict=true)")
	}
	return err
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
