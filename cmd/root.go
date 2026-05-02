package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/checkpoint"
	"github.com/krishyogee/gitmate/internal/config"
	"github.com/krishyogee/gitmate/internal/memory"
	"github.com/krishyogee/gitmate/internal/observability"
	"github.com/krishyogee/gitmate/internal/tools"
	"github.com/krishyogee/gitmate/internal/tui"
)

var (
	flagAuto     bool
	flagDryRun   bool
	flagBase     string
	flagNoAI     bool
	flagVerbose  bool
	flagFriendly bool
)

type App struct {
	Cfg        *config.Config
	Logger     *observability.Logger
	AI         *ai.Client
	Approval   *approval.Manager
	Store      *memory.Store
	Checkpoint *checkpoint.Recorder
	RepoRoot   string
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
		Cfg:        cfg,
		Logger:     logger,
		AI:         client,
		Approval:   approval.NewManager(logger),
		Store:      memory.NewStore(),
		Checkpoint: checkpoint.NewRecorder(root),
		RepoRoot:   root,
	}
	if flagAuto {
		app.Approval.SetAuto(true)
	}
	if flagFriendly {
		app.Cfg.Output.Friendly = true
	}
	return app, nil
}

// Say prints a friendly summary of the given facts when Output.Friendly is on.
// It is a no-op otherwise (the raw command output above is assumed to already
// cover the user). If the AI call fails the raw text is printed as a fallback.
//
// Pass concise, factual text — paths, branches, counts, suggested commands —
// and the AI rephrases into plain easy-to-read prose in Output.Language.
func (a *App) Say(text string) {
	if !a.Cfg.Output.Friendly {
		return
	}
	if flagNoAI || a.AI == nil || !a.AI.HasProvider() || !tui.IsTTY() {
		fmt.Println()
		fmt.Println(tui.Subtle.Render("─── summary ───"))
		fmt.Println(text)
		return
	}
	lang := a.Cfg.Output.Language
	if lang == "" {
		lang = "english"
	}
	system := "You rephrase short CLI tool output into plain, easy-to-understand " + lang + ". " +
		"STRICT RULES:\n" +
		"- Do NOT invent, suggest, or reference any commands, flags, or subcommands. Only repeat command lines that appear VERBATIM in the input.\n" +
		"- Do NOT add 'you can run X' or 'try Y' suggestions of your own.\n" +
		"- Preserve all facts, file paths, commit hashes, branch names, and numbers exactly as given.\n" +
		"- Keep code, paths, and identifiers verbatim; backticks are fine.\n" +
		"- Be concise: 1-3 short sentences. No preface, no markdown fences, no apologies, no follow-up suggestions."
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	out, err := a.AI.Complete(ctx, system, text, "commit_draft")
	fmt.Println()
	fmt.Println(tui.Subtle.Render("─── summary ───"))
	if err != nil || strings.TrimSpace(out) == "" {
		fmt.Println(text)
		return
	}
	fmt.Println(strings.TrimSpace(out))
}

var rootCmd = &cobra.Command{
	Use:   "gitmate",
	Short: "gitmate — AI agent for Git workflows with approval gates",
	Long: `gitmate is a multi-agent CLI that wraps Git with an evaluator-driven AI layer.
Less Git thinking, more shipping — with approvals where it matters.

QUICK START
  gitmate                       open the TUI dashboard (TTY only)
  gitmate init                  set up provider + API key (one-time)
  gitmate ship                  AI commit message + commit + optional PR
  gitmate sync                  fetch + integrate origin/<branch> + base
  gitmate push                  push current branch (with approval)
  gitmate resolve <file>        explain + resolve conflicts block-by-block
  gitmate check                 predict merge pain (overlap + hotspots)
  gitmate explain [file]        plain-language diff summary

CONFIG (no hand-editing JSON)
  gitmate config                show effective config + paths
  gitmate config set defaultBase develop          # repo-local
  gitmate config set syncMode merge --global      # ~/.gitmate/config.json
  gitmate config get models.drafting              # effective value
  gitmate config unset syncMode                   # remove from file

OBSERVABILITY
  gitmate metrics               approval rate, edit rate, latency, scores
  ~/.gitmate/ai-log.jsonl       every AI call + approval logged

PROVIDERS (env or credentials.json — env wins)
  ANTHROPIC_API_KEY · OPENAI_API_KEY · GROQ_API_KEY

DOCS
  https://github.com/krishyogee/gitmate`,
	Run: func(cmd *cobra.Command, args []string) {
		if isFirstRun() {
			printFirstRunBanner()
			_ = cmd.Help()
			return
		}
		if !tui.IsTTY() {
			_ = cmd.Help()
			return
		}
		runDashboard(cmd)
	},
}

func runDashboard(parent *cobra.Command) {
	data := collectDashboardData()
	selected, err := tui.RunDashboard(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dashboard error:", err)
		return
	}
	if selected == "" {
		return
	}
	parts := strings.Fields(selected)
	if len(parts) == 0 {
		return
	}

	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not resolve self path:", err)
		return
	}
	fmt.Printf("\n%s %s\n\n", tui.Subtle.Render("$"), tui.Cmd.Render("gitmate "+selected))

	c := exec.Command(selfPath, parts...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func collectDashboardData() tui.DashboardData {
	ctx := context.Background()
	data := tui.DashboardData{
		Version:   buildVersion,
		Base:      "main",
		RiskLevel: "LOW",
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GROQ_API_KEY"} {
		if os.Getenv(k) != "" {
			data.HasAIKey = true
			break
		}
	}
	if !data.HasAIKey {
		if creds, err := config.LoadCredentials(); err == nil {
			if creds.Anthropic != "" || creds.OpenAI != "" || creds.Groq != "" {
				data.HasAIKey = true
			}
		}
	}
	root, err := tools.RepoRoot(ctx)
	if err != nil || root == "" {
		data.NotInRepo = true
		return data
	}
	data.RepoRoot = root
	cfg, _ := config.Load(root)
	if cfg != nil {
		data.Base = cfg.DefaultBase
	}
	branch, _ := tools.CurrentBranch(ctx)
	data.Branch = branch
	if a, b, err := dashboardAheadBehind(ctx, branch, data.Base); err == nil {
		data.Ahead = a
		data.Behind = b
	}
	if files, err := dashboardChangedFiles(ctx, "HEAD", data.Base); err == nil {
		data.ChangedFiles = len(files)
		if other, err := dashboardChangedFiles(ctx, data.Base, data.Base+"~10"); err == nil {
			set := map[string]bool{}
			for _, f := range files {
				set[f] = true
			}
			for _, f := range other {
				if set[f] {
					data.OverlapCount++
				}
			}
		}
	}
	if data.OverlapCount == 0 {
		data.RiskLevel = "LOW"
	} else if data.OverlapCount >= 3 {
		data.RiskLevel = "HIGH"
	} else {
		data.RiskLevel = "MEDIUM"
	}
	return data
}

func dashboardAheadBehind(ctx context.Context, branch, base string) (int, int, error) {
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

func dashboardChangedFiles(ctx context.Context, ref, base string) ([]string, error) {
	out, err := tools.RunGit(ctx, "diff", "--name-only", base+"..."+ref)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
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
	rootCmd.PersistentFlags().BoolVar(&flagFriendly, "friendly", false, "rephrase output into plain easy-to-read English via AI")

	rootCmd.AddCommand(initCmd, shipCmd, syncCmd, checkCmd, resolveCmd, statusCmd, explainCmd, pushCmd, metricsCmd, configCmd, undoCmd, scheduleCmd, versionCmd)

	silenceUsageOnError(rootCmd)

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

// silenceUsageOnError sets SilenceUsage on the command tree so RunE errors
// don't dump the cobra help block; cobra still prints the error message.
func silenceUsageOnError(c *cobra.Command) {
	c.SilenceUsage = true
	for _, child := range c.Commands() {
		silenceUsageOnError(child)
	}
}
