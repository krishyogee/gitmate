package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/observability"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show metrics derived from ~/.gitmate/ai-log.jsonl (global, all repos)",
	Long: `Aggregate metrics from ~/.gitmate/ai-log.jsonl.

Note: scope is GLOBAL — entries from every repo where you've used gitmate are
included. There is no per-repo filter today (each log entry doesn't carry repo
identity yet). To reset: rm ~/.gitmate/ai-log.jsonl`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := observability.NewLogger()
		m, err := observability.ComputeMetrics(logger.Path())
		if err != nil {
			return err
		}
		fmt.Printf("// source: %s (global scope)\n", logger.Path())
		buf, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(buf))
		return nil
	},
}
