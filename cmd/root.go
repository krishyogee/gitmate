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
	Run: func(cmd *cobra.Command, args []string) {
		if isFirstRun() {
			printFirstRunBanner()
		}
		_ = cmd.Help()
	},
}

func isFirstRun() bool {
	if _, err := os.Stat(config.GlobalPath()); err == nil {
		return false
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GROQ_API_KEY"} {
		if os.Getenv(k) != "" {
			return false
		}
	}
	return true
}

func printFirstRunBanner() {
	fmt.Println("─── first-run detected ───")
	fmt.Println("No config + no API key in env. Run interactive setup:")
	fmt.Println()
	fmt.Println("  gitmate init")
	fmt.Println()
	fmt.Println("Or skip setup and export a key directly:")
	fmt.Println("  export ANTHROPIC_API_KEY=...   # or OPENAI_API_KEY / GROQ_API_KEY")
	fmt.Println("──────────────────────────")
	fmt.Println()
}

func Execute() {
	rootCmd.PersistentFlags().BoolVar(&flagAuto, "auto", false, "auto-approve all actions (use with care)")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "print actions without executing")
	rootCmd.PersistentFlags().StringVar(&flagBase, "base", "", "override default base branch")
	rootCmd.PersistentFlags().BoolVar(&flagNoAI, "no-ai", false, "disable AI calls (fall back to heuristics)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(initCmd, shipCmd, syncCmd, checkCmd, resolveCmd, statusCmd, explainCmd, pushCmd, metricsCmd, configCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

func SetVersion(v, c, d string) {
	buildVersion, buildCommit, buildDate = v, c, d
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print gitmate version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gitmate %s (commit %s, built %s)\n", buildVersion, buildCommit, buildDate)
	},
}
