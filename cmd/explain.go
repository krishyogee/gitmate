package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/tools"
)

var explainStaged bool

var explainCmd = &cobra.Command{
	Use:   "explain [file]",
	Short: "Explain a diff in plain language",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		ctx := context.Background()
		if !app.AI.HasProvider() {
			return fmt.Errorf("no AI provider available. Run `gitmate init` to set up, or export %s_API_KEY",
				strings.ToUpper(app.Cfg.Provider))
		}

		gitArgs := []string{"diff"}
		if explainStaged {
			gitArgs = append(gitArgs, "--cached")
		} else {
			gitArgs = append(gitArgs, "HEAD")
		}
		if len(args) == 1 {
			gitArgs = append(gitArgs, "--", args[0])
		}
		diff, err := tools.RunGit(ctx, gitArgs...)
		if err != nil {
			return err
		}

		if strings.TrimSpace(diff) == "" {
			source := "working tree vs HEAD"
			if explainStaged {
				source = "staged changes (index vs HEAD)"
			}
			fmt.Printf("no diff to explain (%s).\n\nTry one of:\n", source)
			if !explainStaged {
				fmt.Println("  gitmate explain --staged       # explain staged changes (pre-commit)")
			}
			fmt.Println("  gitmate explain <file>         # narrow to one file")
			fmt.Println("  git add <file> && gitmate explain --staged")
			return nil
		}

		recent, _ := tools.GitLogTool{}.Execute(ctx, "5")
		user := fmt.Sprintf("Recent commits:\n%s\n\nDiff:\n%s", recent, ai.SummarizeDiff(diff))

		out, err := app.AI.Complete(ctx, ai.ExplainDiffSystemPrompt, user, "explain")
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	explainCmd.Flags().BoolVar(&explainStaged, "staged", false, "explain staged changes (git diff --cached)")
}
