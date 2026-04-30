package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/config"
	"github.com/krishyogee/gitmate/internal/memory"
	"github.com/krishyogee/gitmate/internal/observability"
	"github.com/krishyogee/gitmate/internal/tools"
)

var (
	flagAuto    bool
	flagDryRun  bool
	flagBase    string
	flagNoAI    bool
	flagVerbose bool
)

type App struct {
	Cfg      *config.Config
	Logger   *observability.Logger
	AI       *ai.Client
	Approval *approval.Manager
	Store    *memory.Store
	RepoRoot string
}

func newApp() (*App, error) {
	root, err := tools.RepoRoot(context.Background())
	if err != nil {
		root = ""
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	if flagBase != "" {
		cfg.DefaultBase = flagBase
	}
	logger := observability.NewLogger()
	client := ai.NewClient(cfg, logger)
	if flagNoAI {
		// keep client; check via HasProvider where needed
	}
	app := &App{
		Cfg:      cfg,
		Logger:   logger,
		AI:       client,
		Approval: approval.NewManager(logger),
		Store:    memory.NewStore(),
		RepoRoot: root,
	}
	if flagAuto {
		app.Approval.SetAuto(true)
	}
	return app, nil
}

var rootCmd = &cobra.Command{
	Use:   "gitmate",
	Short: "gitmate — AI agent for Git workflows with approval gates",
	Long: `gitmate is a multi-agent CLI that wraps Git with an evaluator-driven AI layer.
Less Git thinking, more shipping — with approvals where it matters.`,
}

func Execute() {
	rootCmd.PersistentFlags().BoolVar(&flagAuto, "auto", false, "auto-approve all actions (use with care)")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "print actions without executing")
	rootCmd.PersistentFlags().StringVar(&flagBase, "base", "", "override default base branch")
	rootCmd.PersistentFlags().BoolVar(&flagNoAI, "no-ai", false, "disable AI calls (fall back to heuristics)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(shipCmd, syncCmd, checkCmd, resolveCmd, statusCmd, explainCmd, pushCmd, metricsCmd, configCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print gitmate version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gitmate 0.1.0")
	},
}
