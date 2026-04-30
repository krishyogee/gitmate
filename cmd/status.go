package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/tools"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show branch state + overlap zones + risk indicator",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		ctx := context.Background()
		if app.RepoRoot == "" {
			return fmt.Errorf("not in a git repo")
		}

		branch, _ := tools.CurrentBranch(ctx)
		base := app.Cfg.DefaultBase

		fmt.Printf("branch: %s   base: %s\n", branch, base)

		ahead, behind, _ := aheadBehind(ctx, branch, base)
		fmt.Printf("ahead:  %d   behind: %d\n", ahead, behind)

		st, _ := (tools.GitStatusTool{}).Execute(ctx, "")
		fmt.Println("\n─── working tree ───")
		fmt.Println(st)

		ours, _ := changedFiles(ctx, "HEAD", base)
		theirs, _ := changedFiles(ctx, base, base+"~10")
		overlap := intersect(ours, theirs)
		fmt.Println("─── overlap zones ───")
		if len(overlap) == 0 {
			fmt.Println("  none")
		} else {
			for _, f := range overlap {
				marker := ""
				if app.Cfg.IsHighRiskFile(f) {
					marker = " ⚠"
				}
				fmt.Printf("  %s%s\n", f, marker)
			}
		}

		risk := scoreRisk(overlap, app.Cfg)
		fmt.Printf("\nrisk: %s\n", risk)
		return nil
	},
}

func aheadBehind(ctx context.Context, branch, base string) (int, int, error) {
	out, err := tools.RunGit(ctx, "rev-list", "--left-right", "--count", base+"..."+branch)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected: %s", out)
	}
	var b, a int
	fmt.Sscanf(parts[0], "%d", &b)
	fmt.Sscanf(parts[1], "%d", &a)
	return a, b, nil
}
