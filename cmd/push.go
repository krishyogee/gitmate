package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/tools"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push current branch to origin (with approval)",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		ctx := context.Background()
		if app.RepoRoot == "" {
			return fmt.Errorf("not in a git repo")
		}
		branch, err := tools.CurrentBranch(ctx)
		if err != nil {
			return err
		}

		card := approval.Card{
			Action:      "git_push",
			Input:       fmt.Sprintf("git push -u origin %s", branch),
			Description: fmt.Sprintf("push branch %q to origin", branch),
		}
		dec, _, err := app.Approval.Request(card)
		if err != nil {
			return err
		}
		if dec == approval.DecisionNo {
			return fmt.Errorf("push denied")
		}
		if flagDryRun {
			fmt.Println("[dry-run] would: git push -u origin", branch)
			return nil
		}
		out, err := tools.RunGit(ctx, "push", "-u", "origin", branch)
		fmt.Println(out)
		if err != nil {
			return err
		}
		app.Logger.LogCommand("push", branch, true, nil)
		fmt.Println("✓ pushed")
		return nil
	},
}
