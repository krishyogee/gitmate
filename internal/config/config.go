package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DefaultBase string           `json:"defaultBase"`
	SyncMode    string           `json:"syncMode"`
	AutoStash   bool             `json:"autoStash"`
	Provider    string           `json:"provider"`
	Models      Models           `json:"models"`
	TestCommand string           `json:"testCommand"`
	LintCommand string           `json:"lintCommand"`
	Approval    ApprovalConfig   `json:"approval"`
	Guardrails  GuardrailsConfig `json:"guardrails"`
	Privacy     PrivacyConfig    `json:"privacy"`
	Schedule    ScheduleConfig   `json:"schedule"`
}

type ScheduleConfig struct {
	Enabled    bool     `json:"enabled"`
	Time       string   `json:"time"`
	Timezone   string   `json:"timezone"`
	Repos      []string `json:"repos"`
	OnConflict string   `json:"onConflict"`
	Notify     string   `json:"notify"`
}

type Models struct {
	Planning string `json:"planning"`
	Drafting string `json:"drafting"`
	Fallback string `json:"fallback"`
}

type ApprovalConfig struct {
	TextGeneration  string `json:"textGeneration"`
	PatchGeneration string `json:"patchGeneration"`
	FileWrite       string `json:"fileWrite"`
	GitExecution    string `json:"gitExecution"`
	Shell           string `json:"shell"`
}

type GuardrailsConfig struct {
	MaxLoopSteps                int      `json:"maxLoopSteps"`
	MinConfidenceToApply        float64  `json:"minConfidenceToApply"`
	AlwaysRunTestsAfterConflict bool     `json:"alwaysRunTestsAfterConflict"`
	WarnOnHighRiskFiles         bool     `json:"warnOnHighRiskFiles"`
	HighRiskPatterns            []string `json:"highRiskPatterns"`
}

type PrivacyConfig struct {
	Mode          string `json:"mode"`
	RedactSecrets bool   `json:"redactSecrets"`
}

func Default() *Config {
	return &Config{
		DefaultBase: "main",
		SyncMode:    "rebase",
		AutoStash:   true,
		Provider:    "anthropic",
		Models: Models{
			Planning: "claude-opus-4-7",
			Drafting: "claude-haiku-4-5-20251001",
			Fallback: "claude-sonnet-4-6",
		},
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		Approval: ApprovalConfig{
			TextGeneration:  "ask-once",
			PatchGeneration: "ask",
			FileWrite:       "ask",
			GitExecution:    "ask",
			Shell:           "ask",
		},
		Guardrails: GuardrailsConfig{
			MaxLoopSteps:                6,
			MinConfidenceToApply:        0.65,
			AlwaysRunTestsAfterConflict: true,
			WarnOnHighRiskFiles:         true,
			HighRiskPatterns: []string{
				"auth/", "schema/", "migrations/", ".env",
				"go.sum", "package-lock.json", "secrets/",
			},
		},
		Privacy: PrivacyConfig{
			Mode:          "cloud",
			RedactSecrets: true,
		},
		Schedule: ScheduleConfig{
			Enabled:    false,
			Time:       "08:30",
			Timezone:   "local",
			Repos:      []string{},
			OnConflict: "stop",
			Notify:     "log",
		},
	}
}

func GlobalDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gitmate")
}

func GlobalPath() string {
	return filepath.Join(GlobalDir(), "config.json")
}

func RepoPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".gitmate", "config.json")
}

func Load(repoRoot string) (*Config, error) {
	cfg := Default()

	if err := os.MkdirAll(GlobalDir(), 0755); err != nil {
		return nil, fmt.Errorf("create global dir: %w", err)
	}

	if data, err := os.ReadFile(GlobalPath()); err == nil {
		_ = json.Unmarshal(data, cfg)
	} else if os.IsNotExist(err) {
		_ = Save(cfg, GlobalPath())
	}

	if repoRoot != "" {
		if data, err := os.ReadFile(RepoPath(repoRoot)); err == nil {
			_ = json.Unmarshal(data, cfg)
		}
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GITMATE_PROVIDER"); v != "" {
		cfg.Provider = strings.ToLower(v)
	}
	if v := os.Getenv("GITMATE_PLANNING_MODEL"); v != "" {
		cfg.Models.Planning = v
	}
	if v := os.Getenv("GITMATE_DRAFTING_MODEL"); v != "" {
		cfg.Models.Drafting = v
	}
	if v := os.Getenv("GITMATE_FALLBACK_MODEL"); v != "" {
		cfg.Models.Fallback = v
	}
	if v := os.Getenv("GITMATE_TEST_COMMAND"); v != "" {
		cfg.TestCommand = v
	}
	if v := os.Getenv("GITMATE_LINT_COMMAND"); v != "" {
		cfg.LintCommand = v
	}
	if v := os.Getenv("GITMATE_DEFAULT_BASE"); v != "" {
		cfg.DefaultBase = v
	}
}

func (c *Config) IsHighRiskFile(path string) bool {
	for _, pat := range c.Guardrails.HighRiskPatterns {
		if strings.Contains(path, pat) {
			return true
		}
	}
	return false
}
