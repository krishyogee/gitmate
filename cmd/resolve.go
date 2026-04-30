package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/conflict"
	"github.com/krishyogee/gitmate/internal/memory"
	"github.com/krishyogee/gitmate/internal/tools"
	"time"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve <file>",
	Short: "Explain and resolve conflicts in a file (block by block, with approval)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		ctx := context.Background()

		file := args[0]
		blocks, err := conflict.ParseFile(file)
		if err != nil {
			return err
		}
		if len(blocks) == 0 {
			fmt.Println("(no conflicts found in", file, ")")
			return nil
		}

		fmt.Printf("found %d conflict block(s) in %s\n", len(blocks), file)

		original, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		content := string(original)

		for i, block := range blocks {
			c := conflict.Classify(block)
			fmt.Printf("\n=== Block %d/%d (line %d, complexity=%s) ===\n",
				i+1, len(blocks), block.StartLine, c)

			fmt.Println("--- ours ---")
			fmt.Println(strings.Join(block.OursLines, "\n"))
			fmt.Println("--- theirs ---")
			fmt.Println(strings.Join(block.TheirsLines, "\n"))

			ex, err := conflict.Explain(ctx, app.AI, block)
			if err != nil {
				fmt.Println("AI explain failed:", err)
				fmt.Println("falling back to manual resolution. Skipping block.")
				continue
			}
			renderExplanation(ex)

			if ex.ResolutionStrategy == "manual_required" || ex.CandidatePatch == "" {
				fmt.Println("\nstrategy = manual_required. Skipping block; resolve manually.")
				continue
			}
			if ex.Confidence < app.Cfg.Guardrails.MinConfidenceToApply {
				fmt.Printf("\nconfidence %.2f below minConfidenceToApply (%.2f). Will still ask for approval.\n",
					ex.Confidence, app.Cfg.Guardrails.MinConfidenceToApply)
			}

			card := approval.Card{
				Action:      "resolve_conflict",
				Input:       ex.CandidatePatch,
				Preview:     fmt.Sprintf("strategy=%s\nconfidence=%.2f\nrisk=%s\n\npatch:\n%s",
					ex.ResolutionStrategy, ex.Confidence, ex.RiskNotes, ex.CandidatePatch),
				Description: "apply candidate patch to this conflict block",
			}
			dec, edited, err := app.Approval.Request(card)
			if err != nil {
				return err
			}
			finalPatch := ex.CandidatePatch
			accepted := false
			switch dec {
			case approval.DecisionNo:
				fmt.Println("skipped block.")
				app.Store.RecordConflict(memory.ConflictRecord{
					FilePattern: file, Resolution: ex.ResolutionStrategy,
					UserAccepted: false, Timestamp: time.Now()})
				continue
			case approval.DecisionEdit:
				finalPatch = edited
				accepted = true
			case approval.DecisionYes, approval.DecisionSession:
				accepted = true
			}

			content = applyBlock(content, block, finalPatch)
			fmt.Println("✓ block updated in memory.")
			app.Store.RecordConflict(memory.ConflictRecord{
				FilePattern: file, Resolution: ex.ResolutionStrategy,
				UserAccepted: accepted, Timestamp: time.Now()})
		}

		if strings.Contains(content, "<<<<<<<") || strings.Contains(content, ">>>>>>>") {
			fmt.Println("\n⚠ unresolved conflict markers remain — file not written.")
			return nil
		}

		card := approval.Card{
			Action:      "write_file",
			Input:       file,
			Preview:     truncatePreview(content),
			Description: fmt.Sprintf("write resolved content to %s and stage", file),
		}
		dec, _, err := app.Approval.Request(card)
		if err != nil {
			return err
		}
		if dec == approval.DecisionNo {
			fmt.Println("aborted file write.")
			return nil
		}
		if flagDryRun {
			fmt.Println("[dry-run] would write", file)
			return nil
		}

		input := file + "\n" + content
		out, err := (tools.ResolveConflictTool{}).Execute(ctx, input)
		if err != nil {
			return err
		}
		fmt.Println(out)

		if app.Cfg.Guardrails.AlwaysRunTestsAfterConflict && app.Cfg.TestCommand != "" {
			fmt.Println("\nrunning tests...")
			testOut, terr := (tools.RunTestsTool{Command: app.Cfg.TestCommand}).Execute(ctx, "")
			fmt.Println(testOut)
			if terr != nil {
				fmt.Println("⚠ tests failed:", terr)
			} else {
				fmt.Println("✓ tests passed")
			}
		}

		fmt.Println("\nNext: continue your rebase/merge manually:")
		fmt.Println("  git rebase --continue   # or git merge --continue")
		return nil
	},
}

func renderExplanation(ex *conflict.Explanation) {
	fmt.Println("\n--- analysis ---")
	fmt.Println("ours intent:    ", ex.OursIntent)
	fmt.Println("theirs intent:  ", ex.TheirsIntent)
	fmt.Println("conflict type:  ", ex.ConflictType)
	fmt.Println("strategy:       ", ex.ResolutionStrategy)
	fmt.Printf("confidence:      %.2f\n", ex.Confidence)
	fmt.Println("rationale:      ", ex.ResolutionRationale)
	fmt.Println("risk:           ", ex.RiskNotes)
	if ex.CandidatePatch != "" {
		fmt.Println("\n--- candidate patch ---")
		fmt.Println(ex.CandidatePatch)
	}
}

func applyBlock(content string, block conflict.Block, patch string) string {
	lines := strings.Split(content, "\n")
	if block.EndLine > len(lines) || block.StartLine < 1 {
		return content
	}
	before := lines[:block.StartLine-1]
	after := lines[block.EndLine:]
	patchLines := strings.Split(patch, "\n")
	out := append([]string{}, before...)
	out = append(out, patchLines...)
	out = append(out, after...)
	return strings.Join(out, "\n")
}

func truncatePreview(s string) string {
	if len(s) <= 1500 {
		return s
	}
	return s[:1500] + "\n…(truncated)"
}
