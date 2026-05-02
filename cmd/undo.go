package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/checkpoint"
	"github.com/krishyogee/gitmate/internal/tools"
)

var (
	undoSteps   int
	undoID      string
	undoForce   bool
	undoDryRun  bool
	undoHard    bool
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Reverse the most recent gitmate operation (commit, rebase, merge, push, stash, file write, PR)",
	Long: `Reverse recorded gitmate operations.

Each mutating gitmate command writes a checkpoint to <repo>/.gitmate/checkpoints.json.
'gitmate undo' pops the most recent done+reversible checkpoint and reverses it.

  gitmate undo                        undo last reversible op
  gitmate undo --steps 3              undo last 3 ops (newest first)
  gitmate undo --id <checkpoint-id>   undo specific checkpoint
  gitmate undo list                   show recent checkpoints
  gitmate undo --dry-run              show what would happen
  gitmate undo --force                allow undoing pushed commits (force-with-lease)
  gitmate undo --hard                 'git reset --hard' on commit undo (default soft)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		if app.RepoRoot == "" {
			return fmt.Errorf("not in a git repo")
		}
		if app.Checkpoint == nil {
			return fmt.Errorf("checkpoint store unavailable")
		}
		ctx := context.Background()

		if len(args) == 1 && args[0] == "list" {
			return printCheckpointList(app.Checkpoint.Store)
		}

		if undoID != "" {
			op, err := app.Checkpoint.Store.Get(undoID)
			if err != nil {
				return err
			}
			return undoOne(ctx, app, op)
		}

		steps := undoSteps
		if steps < 1 {
			steps = 1
		}
		ops, err := app.Checkpoint.Store.List()
		if err != nil {
			return err
		}
		picked := []checkpoint.Op{}
		for _, op := range ops {
			if op.Status != "done" {
				continue
			}
			picked = append(picked, op)
			if len(picked) >= steps {
				break
			}
		}
		if len(picked) == 0 {
			fmt.Println("no checkpoints to undo.")
			return nil
		}
		for i := range picked {
			if err := undoOne(ctx, app, &picked[i]); err != nil {
				return err
			}
		}
		return nil
	},
}

func printCheckpointList(s *checkpoint.Store) error {
	ops, err := s.List()
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		fmt.Println("(no checkpoints)")
		return nil
	}
	limit := 20
	if len(ops) < limit {
		limit = len(ops)
	}
	fmt.Printf("%-32s  %-10s  %-12s  %-8s  %s\n", "ID", "COMMAND", "OP", "STATUS", "INFO")
	for i := 0; i < limit; i++ {
		op := ops[i]
		info := ""
		switch op.OpType {
		case "commit":
			info = shortSha(op.HeadAfter)
		case "rebase", "merge":
			info = shortSha(op.HeadBefore) + "→" + shortSha(op.HeadAfter)
		case "push":
			info = op.RemoteBranch + "@" + shortSha(op.RemoteSHABefore)
		case "pr_create":
			info = "PR " + op.PRNumber
		case "stash":
			info = shortSha(op.StashRef)
		case "file_write":
			if len(op.FilesWritten) > 0 {
				info = op.FilesWritten[0].Path
			}
		}
		flag := ""
		if !op.Reversible {
			flag = " [irreversible]"
		}
		fmt.Printf("%-32s  %-10s  %-12s  %-8s  %s%s\n", op.ID, op.Command, op.OpType, op.Status, info, flag)
	}
	return nil
}

func shortSha(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func undoOne(ctx context.Context, app *App, op *checkpoint.Op) error {
	if op.Status == "undone" {
		return fmt.Errorf("checkpoint %s already undone", op.ID)
	}
	if !op.Reversible && !undoForce {
		return fmt.Errorf("checkpoint %s marked irreversible: %s (use --force to attempt anyway)", op.ID, op.ReasonIfNot)
	}

	desc := fmt.Sprintf("undo %s/%s [%s]", op.Command, op.OpType, op.ID)
	fmt.Println("─── " + desc + " ───")

	planned, err := planUndo(op)
	if err != nil {
		return err
	}
	for _, line := range planned.summary {
		fmt.Println("  " + line)
	}
	if undoDryRun {
		fmt.Println("[dry-run] no changes made.")
		return nil
	}

	card := approval.Card{
		Action:      "git_apply",
		Input:       desc,
		Description: "reverse this operation",
		Preview:     strings.Join(planned.summary, "\n"),
	}
	dec, _, err := app.Approval.Request(card)
	if err != nil {
		return err
	}
	if dec == approval.DecisionNo {
		return fmt.Errorf("undo aborted")
	}

	if err := planned.run(ctx, app); err != nil {
		return fmt.Errorf("undo %s: %w", op.ID, err)
	}
	op.Status = "undone"
	if err := app.Checkpoint.Store.Update(*op); err != nil {
		return fmt.Errorf("mark undone: %w", err)
	}
	fmt.Println("✓ undone")
	return nil
}

type undoPlan struct {
	summary []string
	run     func(ctx context.Context, app *App) error
}

func planUndo(op *checkpoint.Op) (*undoPlan, error) {
	switch op.OpType {
	case "commit":
		mode := "--soft"
		if undoHard {
			mode = "--hard"
		}
		target := op.HeadBefore
		if target == "" {
			return nil, fmt.Errorf("no head_before recorded — cannot undo commit %s", op.ID)
		}
		return &undoPlan{
			summary: []string{fmt.Sprintf("git reset %s %s", mode, shortSha(target))},
			run: func(ctx context.Context, app *App) error {
				_, err := tools.RunGit(ctx, "reset", mode, target)
				return err
			},
		}, nil
	case "rebase", "merge":
		target := op.HeadBefore
		if target == "" && op.BackupRef != "" {
			target = op.BackupRef
		}
		if target == "" {
			return nil, fmt.Errorf("no head_before / backup_ref — cannot undo %s %s", op.OpType, op.ID)
		}
		return &undoPlan{
			summary: []string{fmt.Sprintf("git reset --hard %s", shortSha(target))},
			run: func(ctx context.Context, app *App) error {
				if _, err := tools.RunGit(ctx, "reset", "--hard", target); err != nil {
					return err
				}
				if op.BackupRef != "" {
					_, _ = tools.RunGit(ctx, "update-ref", "-d", op.BackupRef)
				}
				return nil
			},
		}, nil
	case "stash":
		ref := op.StashRef
		if ref == "" {
			ref = "stash@{0}"
		}
		return &undoPlan{
			summary: []string{fmt.Sprintf("git stash pop %s", shortSha(ref))},
			run: func(ctx context.Context, app *App) error {
				_, err := tools.RunGit(ctx, "stash", "pop", ref)
				return err
			},
		}, nil
	case "push":
		if op.RemoteSHABefore == "" {
			return nil, fmt.Errorf("first push has no prior remote sha — nothing to roll back to")
		}
		if !undoForce {
			return nil, fmt.Errorf("undoing a push rewrites remote history; pass --force to confirm")
		}
		remote := op.Remote
		if remote == "" {
			remote = "origin"
		}
		branch := op.RemoteBranch
		if branch == "" {
			branch = op.Branch
		}
		spec := fmt.Sprintf("%s:refs/heads/%s", op.RemoteSHABefore, branch)
		lease := fmt.Sprintf("--force-with-lease=%s", branch)
		return &undoPlan{
			summary: []string{fmt.Sprintf("git push %s %s %s (rewrites %s/%s)", lease, remote, spec, remote, branch)},
			run: func(ctx context.Context, app *App) error {
				_, err := tools.RunGit(ctx, "push", lease, remote, spec)
				return err
			},
		}, nil
	case "pr_create":
		num := op.PRNumber
		if num == "" {
			return nil, fmt.Errorf("no PR number recorded")
		}
		return &undoPlan{
			summary: []string{fmt.Sprintf("gh pr close %s", num)},
			run: func(ctx context.Context, app *App) error {
				if _, err := exec.LookPath("gh"); err != nil {
					return fmt.Errorf("gh CLI not installed")
				}
				cmd := exec.CommandContext(ctx, "gh", "pr", "close", num)
				out, err := cmd.CombinedOutput()
				fmt.Println(string(out))
				return err
			},
		}, nil
	case "file_write":
		if len(op.FilesWritten) == 0 {
			return nil, fmt.Errorf("no file backups recorded")
		}
		var lines []string
		for _, fb := range op.FilesWritten {
			lines = append(lines, fmt.Sprintf("restore %s from %s", fb.Path, fb.BackupPath))
		}
		return &undoPlan{
			summary: lines,
			run: func(ctx context.Context, app *App) error {
				for _, fb := range op.FilesWritten {
					if err := restoreFile(fb, app.RepoRoot); err != nil {
						return err
					}
				}
				return nil
			},
		}, nil
	}
	return nil, fmt.Errorf("unsupported op type: %s", op.OpType)
}

func restoreFile(fb checkpoint.FileBackup, repoRoot string) error {
	src, err := os.Open(fb.BackupPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dstPath := fb.Path
	if !isAbs(dstPath) {
		dstPath = repoRoot + string(os.PathSeparator) + dstPath
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func isAbs(p string) bool {
	return strings.HasPrefix(p, "/") || (len(p) > 1 && p[1] == ':')
}

func init() {
	undoCmd.Flags().IntVar(&undoSteps, "steps", 1, "number of recent ops to undo")
	undoCmd.Flags().StringVar(&undoID, "id", "", "undo a specific checkpoint by id")
	undoCmd.Flags().BoolVar(&undoForce, "force", false, "allow undoing pushed commits (force-with-lease)")
	undoCmd.Flags().BoolVar(&undoDryRun, "dry-run", false, "show what would happen without executing")
	undoCmd.Flags().BoolVar(&undoHard, "hard", false, "use git reset --hard for commit undo (default --soft)")
}
