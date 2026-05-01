package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/config"
	"github.com/krishyogee/gitmate/internal/tools"
)

var configGlobal bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or modify gitmate config",
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

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value (writes to repo config by default; --global for ~/.gitmate)",
	Args:  cobra.ExactArgs(2),
	Example: `  gitmate config set defaultBase develop
  gitmate config set syncMode merge
  gitmate config set models.drafting gpt-4o-mini
  gitmate config set guardrails.maxLoopSteps 8 --global`,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, raw := args[0], args[1]
		path, err := chooseConfigPath(configGlobal)
		if err != nil {
			return err
		}

		data, err := loadConfigMap(path)
		if err != nil {
			return err
		}
		if err := setNested(data, strings.Split(key, "."), parseValue(raw)); err != nil {
			return err
		}
		if err := writeConfigMap(path, data); err != nil {
			return err
		}
		fmt.Printf("✓ %s = %s in %s\n", key, raw, path)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print a single config value (effective, after layering)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		buf, _ := json.Marshal(app.Cfg)
		var m map[string]any
		if err := json.Unmarshal(buf, &m); err != nil {
			return err
		}
		val, ok := getNested(m, strings.Split(args[0], "."))
		if !ok {
			return fmt.Errorf("key %q not found", args[0])
		}
		switch v := val.(type) {
		case string, bool, float64, int:
			fmt.Println(v)
		default:
			out, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(out))
		}
		return nil
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a key from repo (or --global) config file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := chooseConfigPath(configGlobal)
		if err != nil {
			return err
		}
		data, err := loadConfigMap(path)
		if err != nil {
			return err
		}
		if err := unsetNested(data, strings.Split(args[0], ".")); err != nil {
			return err
		}
		if err := writeConfigMap(path, data); err != nil {
			return err
		}
		fmt.Printf("✓ unset %s in %s\n", args[0], path)
		return nil
	},
}

func chooseConfigPath(forceGlobal bool) (string, error) {
	if forceGlobal {
		return config.GlobalPath(), nil
	}
	root, err := tools.RepoRoot(context.Background())
	if err == nil && root != "" {
		return config.RepoPath(root), nil
	}
	return config.GlobalPath(), nil
}

func loadConfigMap(path string) (map[string]any, error) {
	data := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return data, nil
}

func writeConfigMap(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

func setNested(m map[string]any, keys []string, value any) error {
	if len(keys) == 0 {
		return fmt.Errorf("empty key")
	}
	if len(keys) == 1 {
		m[keys[0]] = value
		return nil
	}
	next, ok := m[keys[0]].(map[string]any)
	if !ok {
		next = map[string]any{}
		m[keys[0]] = next
	}
	return setNested(next, keys[1:], value)
}

func unsetNested(m map[string]any, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("empty key")
	}
	if len(keys) == 1 {
		delete(m, keys[0])
		return nil
	}
	next, ok := m[keys[0]].(map[string]any)
	if !ok {
		return nil
	}
	return unsetNested(next, keys[1:])
}

func getNested(m map[string]any, keys []string) (any, bool) {
	if len(keys) == 0 {
		return nil, false
	}
	v, ok := m[keys[0]]
	if !ok {
		return nil, false
	}
	if len(keys) == 1 {
		return v, true
	}
	next, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return getNested(next, keys[1:])
}

func parseValue(raw string) any {
	switch strings.ToLower(raw) {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	if strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "{") {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			return v
		}
	}
	return raw
}

func init() {
	configSetCmd.Flags().BoolVar(&configGlobal, "global", false, "write to ~/.gitmate/config.json instead of repo config")
	configUnsetCmd.Flags().BoolVar(&configGlobal, "global", false, "modify ~/.gitmate/config.json instead of repo config")
	configCmd.AddCommand(configSetCmd, configGetCmd, configUnsetCmd)
}
