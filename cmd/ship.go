package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/agent"
	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/tools"
	"github.com/krishyogee/gitmate/internal/tui"
)

var (
	shipNoPR     bool
	shipPushOnly bool
)

var shipCmd = &cobra.Command{
	Use:   "ship",
	Short: "Generate commit message + commit + optional PR from staged changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		if app.RepoRoot == "" {
			return fmt.Errorf("not inside a git repository")
		}
		ctx := context.Background()

		stream := tui.NewStream()

		stream.Start("reading staged diff")
		diff, err := tools.GitDiffTool{}.Execute(ctx, "")
		if err != nil {
			stream.Fail("git diff failed")
			return err
		}
		if strings.Contains(diff, "(no changes)") {
			stream.Fail("nothing staged")
			return fmt.Errorf("nothing staged. Run `git add` first")
		}
		stream.Done(fmt.Sprintf("staged diff ready (%d chars)", len(diff)))

		fmt.Println()
		fmt.Println(tui.Subtle.Render("─── staged diff (compressed if large) ───"))
		fmt.Println(diff)
		fmt.Println(tui.Subtle.Render("─────────────────────────────────────────"))

		if !app.AI.HasProvider() {
			return fmt.Errorf("no AI provider available. Run `gitmate init` to set up, or export %s_API_KEY",
				strings.ToUpper(app.Cfg.Provider))
		}

		evaluator := agent.CommitEvaluator{}
		ctxText := app.Store.RepoContext(app.RepoRoot)

		stream.Start("drafting commit message")
		userPrompt := fmt.Sprintf("Repo context: %s\n\nStaged diff:\n%s", ctxText, diff)
		message, err := app.AI.Complete(ctx, ai.CommitDraftSystemPrompt, userPrompt, "commit_draft")
		if err != nil {
			stream.Fail("draft failed")
			return fmt.Errorf("draft commit: %w", err)
		}
		message = strings.TrimSpace(message)

		score := evaluator.Score("generate_commit", message)
		stream.Done(fmt.Sprintf("draft scored %s", scoreLabel(score)))
		app.Logger.LogStep("ship", "generate_commit", message, score)

		if score < evaluator.PassThreshold() {
			stream.Start("refining commit message")
			refinePrompt := fmt.Sprintf("Diff:\n%s\n\nPrevious draft:\n%s", diff, message)
			refined, rerr := app.AI.Complete(ctx, ai.CommitRefineSystemPrompt, refinePrompt, "commit_draft")
			if rerr == nil {
				refined = strings.TrimSpace(refined)
				rscore := evaluator.Score("refine_commit", refined)
				app.Logger.LogStep("ship", "refine_commit", refined, rscore)
				if rscore > score {
					message = refined
					score = rscore
					stream.Done(fmt.Sprintf("refined to %s", scoreLabel(rscore)))
				} else {
					stream.Info(fmt.Sprintf("refine kept original (%s vs %s)", scoreLabel(rscore), scoreLabel(score)))
				}
			} else {
				stream.Fail("refine call errored")
			}
		}

		card := approval.Card{
			Action:      "git_commit",
			Input:       message,
			Description: "commit staged changes with this message",
		}
		dec, edited, err := app.Approval.Request(card)
		if err != nil {
			return err
		}
		switch dec {
		case approval.DecisionNo:
			return fmt.Errorf("commit denied by user")
		case approval.DecisionEdit:
			message = strings.TrimSpace(edited)
		}

		if flagDryRun {
			fmt.Println("\n[dry-run] would commit with message:")
			fmt.Println(message)
			return nil
		}

		stream.Start("committing")
		out, err := tools.GitCommitTool{}.Execute(ctx, message)
		if err != nil {
			stream.Fail("git commit failed")
			return fmt.Errorf("git commit: %w", err)
		}
		stream.Done("commit landed")
		fmt.Println(tui.Subtle.Render(strings.TrimSpace(out)))

		if shipNoPR {
			return nil
		}

		fmt.Printf("\n%s ", tui.Hint.Render("Create PR? (y/N)"))
		r := approval.SharedStdin()
		ans, _ := r.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(ans)) != "y" {
			return nil
		}

		return runShipPR(ctx, app, message)
	},
}

func runShipPR(ctx context.Context, app *App, lastCommit string) error {
	branch, err := tools.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	if branch == app.Cfg.DefaultBase {
		return fmt.Errorf("on base branch %q — switch branches before opening PR", branch)
	}

	commits, _ := tools.GitLogTool{}.Execute(ctx, "20")
	diff, _ := tools.GitDiffTool{}.Execute(ctx, "")

	user := fmt.Sprintf("Branch: %s\nBase: %s\nLatest commit: %s\nRecent commits:\n%s\nDiff summary:\n%s",
		branch, app.Cfg.DefaultBase, lastCommit, commits, diff)

	raw, err := app.AI.Complete(ctx, ai.PRDraftSystemPrompt, user, "pr_draft")
	if err != nil {
		return fmt.Errorf("draft PR: %w", err)
	}
	title, body := parsePRDraft(raw)
	if title == "" {
		return fmt.Errorf("could not parse PR draft: %s", raw)
	}

	preview := fmt.Sprintf("title: %s\n\nbody:\n%s", title, body)
	card := approval.Card{
		Action:      "create_pr",
		Input:       title + "\n\n" + body,
		Preview:     preview,
		Description: "create draft PR via gh",
	}
	dec, edited, err := app.Approval.Request(card)
	if err != nil {
		return err
	}
	if dec == approval.DecisionNo {
		return nil
	}
	finalInput := card.Input
	if dec == approval.DecisionEdit {
		finalInput = edited
	}

	fmt.Println("pushing branch...")
	if out, err := tools.PushBranch(ctx); err != nil {
		fmt.Println(out)
		return err
	}

	if flagDryRun {
		fmt.Println("[dry-run] would create PR:")
		fmt.Println(finalInput)
		return nil
	}

	out, err := (tools.CreatePRTool{Base: app.Cfg.DefaultBase}).Execute(ctx, finalInput)
	if err != nil {
		fmt.Println(out)
		return err
	}
	fmt.Println(out)
	return nil
}

func parsePRDraft(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		idx := strings.Index(raw, "\n")
		if idx > 0 {
			raw = raw[idx+1:]
		}
		raw = strings.TrimSuffix(raw, "```")
	}
	type pr struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	var p pr
	if err := jsonUnmarshalLenient(raw, &p); err == nil && p.Title != "" {
		return p.Title, p.Body
	}
	return "", raw
}

func init() {
	shipCmd.Flags().BoolVar(&shipNoPR, "no-pr", false, "skip PR creation")
	shipCmd.Flags().BoolVar(&shipPushOnly, "push", false, "only push, do not create PR")
}
