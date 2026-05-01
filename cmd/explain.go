package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/tools"
)

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

		var diff string
		if len(args) == 1 {
			out, err := tools.RunGit(ctx, "diff", "HEAD", "--", args[0])
			if err != nil {
				return err
			}
			diff = out
		} else {
			out, err := tools.RunGit(ctx, "diff", "HEAD")
			if err != nil {
				return err
			}
			diff = out
		}
		if diff == "" {
			fmt.Println("(no diff)")
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
