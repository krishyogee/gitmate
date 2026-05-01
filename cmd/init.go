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
		fmt.Println("Interactive setup. Press Ctrl+C anytime to abort.")
		fmt.Println()
		fmt.Println("This will:")
		fmt.Println("  1. write ~/.gitmate/config.json")
		fmt.Println("  2. append `export <PROVIDER>_API_KEY=...` to your shell rc")
		fmt.Println()

		r := bufio.NewReader(os.Stdin)

		var provider, envVar string
		for {
			fmt.Print("Provider [anthropic / openai / groq] (default: anthropic): ")
			line, err := r.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read provider: %w", err)
			}
			provider = strings.TrimSpace(strings.ToLower(line))
			if provider == "" {
				provider = "anthropic"
			}
			switch provider {
			case "anthropic":
				envVar = "ANTHROPIC_API_KEY"
			case "openai":
				envVar = "OPENAI_API_KEY"
			case "groq":
				envVar = "GROQ_API_KEY"
			default:
				fmt.Printf("  ✗ unknown provider %q. Type one of: anthropic, openai, groq.\n\n", provider)
				continue
			}
			break
		}

		var apiKey string
		for {
			fmt.Printf("\nAPI key for %s (paste key, then Enter): ", envVar)
			line, err := r.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read api key: %w", err)
			}
			apiKey = strings.TrimSpace(line)
			if apiKey == "" {
				fmt.Println("  ✗ key is empty. Paste your API key, or Ctrl+C to cancel.")
				continue
			}
			if strings.Contains(apiKey, " ") || strings.HasPrefix(apiKey, "gitmate ") {
				fmt.Printf("  ✗ that doesn't look like a key (%q). Try again.\n", apiKey)
				continue
			}
			break
		}

		shellRC := detectShellRC()
		fmt.Printf("\nShell rc file (default: %s) [Enter to accept]: ", shellRC)
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

		creds, _ := config.LoadCredentials()
		creds.Set(provider, apiKey)
		if err := config.SaveCredentials(creds); err != nil {
			return fmt.Errorf("write credentials: %w", err)
		}
		fmt.Printf("✓ wrote %s (mode 0600)\n", config.CredentialsPath())

		exportLine := fmt.Sprintf("export %s=%s", envVar, apiKey)
		if err := appendIfMissing(shellRC, exportLine); err != nil {
			fmt.Printf("⚠ couldn't update %s: %v\n", shellRC, err)
		} else {
			fmt.Printf("✓ appended export to %s (optional fallback)\n", shellRC)
		}

		os.Setenv(envVar, apiKey)
		fmt.Println()
		fmt.Println("─── verify ───")

		buf, _ := json.MarshalIndent(struct {
			Provider string `json:"provider"`
			Models   any    `json:"models"`
		}{Provider: cfg.Provider, Models: cfg.Models}, "", "  ")
		fmt.Println(string(buf))

		fmt.Println("\nNext:")
		fmt.Println("  gitmate ship --no-pr   # try it on a repo with staged changes")
		fmt.Println()
		fmt.Printf("(key persisted in %s — works in any shell, no `source` needed)\n", config.CredentialsPath())
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
