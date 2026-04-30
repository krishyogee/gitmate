package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/observability"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show metrics derived from ~/.gitmate/ai-log.jsonl",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := observability.NewLogger()
		m, err := observability.ComputeMetrics(logger.Path())
		if err != nil {
			return err
		}
		buf, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(buf))
		return nil
	},
}
