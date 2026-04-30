package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive first-run setup (provider, API key, shell rc)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("─── gitmate init ───")
		fmt.Println("This will:")
		fmt.Println("  1. write ~/.gitmate/config.json")
		fmt.Println("  2. append `export <PROVIDER>_API_KEY=...` to your shell rc")
		fmt.Println("  3. verify the key by running `gitmate version`")
		fmt.Println()

		r := bufio.NewReader(os.Stdin)

		fmt.Print("Provider [anthropic/openai/groq] (default anthropic): ")
		providerLine, _ := r.ReadString('\n')
		provider := strings.TrimSpace(strings.ToLower(providerLine))
		if provider == "" {
			provider = "anthropic"
		}
		envVar := ""
		switch provider {
		case "anthropic":
			envVar = "ANTHROPIC_API_KEY"
		case "openai":
			envVar = "OPENAI_API_KEY"
		case "groq":
			envVar = "GROQ_API_KEY"
		default:
			return fmt.Errorf("unknown provider %q", provider)
		}

		fmt.Printf("\nAPI key (%s): ", envVar)
		keyLine, _ := r.ReadString('\n')
		apiKey := strings.TrimSpace(keyLine)
		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}

		shellRC := detectShellRC()
		fmt.Printf("\nShell rc file (default %s): ", shellRC)
		rcLine, _ := r.ReadString('\n')
		if v := strings.TrimSpace(rcLine); v != "" {
			shellRC = expandHome(v)
		}

		cfg := config.Default()
		cfg.Provider = provider
		switch provider {
		case "openai":
			cfg.Models.Planning = "gpt-4o"
			cfg.Models.Drafting = "gpt-4o-mini"
			cfg.Models.Fallback = "gpt-4o-mini"
		case "groq":
			cfg.Models.Planning = "llama-3.3-70b-versatile"
			cfg.Models.Drafting = "llama-3.3-70b-versatile"
			cfg.Models.Fallback = "llama-3.3-70b-versatile"
		}

		if err := config.Save(cfg, config.GlobalPath()); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("✓ wrote %s\n", config.GlobalPath())

		exportLine := fmt.Sprintf("export %s=%s", envVar, apiKey)
		if err := appendIfMissing(shellRC, exportLine); err != nil {
			return fmt.Errorf("update %s: %w", shellRC, err)
		}
		fmt.Printf("✓ appended export to %s\n", shellRC)

		os.Setenv(envVar, apiKey)
		fmt.Println()
		fmt.Println("─── verify ───")

		buf, _ := json.MarshalIndent(struct {
			Provider string `json:"provider"`
			Models   any    `json:"models"`
		}{Provider: cfg.Provider, Models: cfg.Models}, "", "  ")
		fmt.Println(string(buf))

		fmt.Println("\nNext:")
		fmt.Printf("  source %s   # or restart your shell\n", shellRC)
		fmt.Println("  gitmate ship --no-pr   # try it on a repo with staged changes")
		return nil
	},
}

func detectShellRC() string {
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return filepath.Join(home, ".zshrc")
	}
	if strings.Contains(shell, "bash") {
		return filepath.Join(home, ".bashrc")
	}
	if strings.Contains(shell, "fish") {
		return filepath.Join(home, ".config", "fish", "config.fish")
	}
	return filepath.Join(home, ".profile")
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func appendIfMissing(path, line string) error {
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), line) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		fmt.Fprintln(f)
	}
	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "# gitmate")
	fmt.Fprintln(f, line)
	return nil
}
