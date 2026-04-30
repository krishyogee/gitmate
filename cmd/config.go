package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show effective gitmate config",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		buf, _ := json.MarshalIndent(app.Cfg, "", "  ")
		fmt.Println("─── effective config ───")
		fmt.Println(string(buf))
		fmt.Println("\n─── paths ───")
		fmt.Println("global:", config.GlobalPath())
		if app.RepoRoot != "" {
			fmt.Println("repo:  ", config.RepoPath(app.RepoRoot))
		}
		fmt.Println("logs:  ", app.Logger.Path())
		return nil
	},
}
