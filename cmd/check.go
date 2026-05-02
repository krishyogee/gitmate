package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/tools"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Predict merge pain before it happens (overlap + hotspots)",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		ctx := context.Background()
		base := app.Cfg.DefaultBase

		fmt.Printf("base: %s\n\n", base)

		if _, err := (tools.GitFetchTool{}).Execute(ctx, ""); err != nil {
			fmt.Printf("warn: fetch failed (%v) — using local refs only\n\n", err)
		}

		branch, _ := tools.CurrentBranch(ctx)
		compareBase := base
		if branch == base {
			remoteRef := "origin/" + base
			if _, err := tools.RunGit(ctx, "rev-parse", "--verify", remoteRef); err == nil {
				compareBase = remoteRef
				fmt.Printf("on base branch — comparing against %s\n\n", remoteRef)
			}
		}

		if ahead, behind, err := compareRefs(ctx, "HEAD", compareBase); err == nil {
			fmt.Printf("vs %s — ahead=%d behind=%d\n\n", compareBase, ahead, behind)
		}

		fmt.Println("─── recent activity on base ───")
		recent, _ := (tools.GitLogTool{}).Execute(ctx, "10")
		fmt.Println(recent)

		ours, _ := changedFiles(ctx, "HEAD", compareBase)
		theirs, _ := changedFiles(ctx, compareBase, fmt.Sprintf("%s~10", compareBase))
		overlap := intersect(ours, theirs)

		fmt.Println("\n─── changed files (yours) ───")
		for _, f := range ours {
			fmt.Println(" ", f)
		}

		fmt.Println("\n─── changed files (base, last 10 commits) ───")
		for _, f := range theirs {
			fmt.Println(" ", f)
		}

		fmt.Println("\n─── overlap zones ───")
		if len(overlap) == 0 {
			fmt.Println("  none")
		} else {
			for _, f := range overlap {
				highRisk := app.Cfg.IsHighRiskFile(f)
				marker := ""
				if highRisk {
					marker = " ⚠ HIGH-RISK"
				}
				fmt.Printf("  %s%s\n", f, marker)
			}
		}

		fmt.Println("\n─── hotspot files on base ───")
		hot, _ := (tools.FetchHotspotsTool{}).Execute(ctx, base)
		fmt.Println(hot)

		risk := scoreRisk(overlap, app.Cfg)
		fmt.Println("─── risk ───")
		fmt.Printf("level: %s\n", risk)

		overlapList := "none"
		if len(overlap) > 0 {
			overlapList = strings.Join(overlap, ", ")
		}
		app.Say(fmt.Sprintf(
			"merge-risk check vs base=%s. risk=%s. yours_changed=%d files, base_changed=%d files, overlap=%d files (%s)",
			base, risk, len(ours), len(theirs), len(overlap), overlapList,
		))
		return nil
	},
}

func changedFiles(ctx context.Context, ref, base string) ([]string, error) {
	out, err := tools.GitLogTool{}.Execute(ctx, "1")
	_ = out
	_ = err
	cmd := []string{"diff", "--name-only", base + "..." + ref}
	o, err := runGitCmd(ctx, cmd...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(o, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func runGitCmd(ctx context.Context, args ...string) (string, error) {
	return tools.RunGit(ctx, args...)
}

func intersect(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	var out []string
	for _, x := range b {
		if set[x] {
			out = append(out, x)
		}
	}
	return out
}

func scoreRisk(overlap []string, cfg interface {
	IsHighRiskFile(string) bool
}) string {
	if len(overlap) == 0 {
		return "LOW"
	}
	for _, f := range overlap {
		if cfg.IsHighRiskFile(f) {
			return "HIGH"
		}
	}
	if len(overlap) >= 3 {
		return "MEDIUM"
	}
	return "LOW"
}
