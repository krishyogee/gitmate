package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/krishyogee/gitmate/internal/config"
	"github.com/krishyogee/gitmate/internal/tools"
)

var (
	scheduleTime    string
	scheduleEnable  bool
	scheduleDisable bool
	schedulePrint   bool
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Configure morning auto-sync (OS scheduler runs `gitmate sync --auto --all`)",
	Long: `Daily auto-sync, fired by your OS scheduler.

  gitmate schedule status               show current config + next fire
  gitmate schedule set --time 08:30     change daily fire time
  gitmate schedule add-repo <path>      add repo to sync list
  gitmate schedule remove-repo <path>   remove repo
  gitmate schedule run-now              fire now (test) — runs sync --auto --all
  gitmate schedule install              register OS scheduler (launchd/systemd)
  gitmate schedule install --print      print service file without installing
  gitmate schedule uninstall            unregister OS scheduler

Schedule lives in ~/.gitmate/config.json under "schedule".
Scheduled run = 'gitmate sync --auto --all'. Each op writes a checkpoint, so you
can 'gitmate undo' anything the morning sync did.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return scheduleStatus()
	},
}

func scheduleStatus() error {
	app, err := newApp()
	if err != nil {
		return err
	}
	s := app.Cfg.Schedule
	fmt.Println("─── schedule ───")
	fmt.Printf("enabled:     %v\n", s.Enabled)
	fmt.Printf("time:        %s (%s)\n", s.Time, s.Timezone)
	fmt.Printf("on-conflict: %s\n", s.OnConflict)
	fmt.Printf("notify:      %s\n", s.Notify)
	fmt.Println("repos:")
	if len(s.Repos) == 0 {
		fmt.Println("  (none — add with `gitmate schedule add-repo <path>`)")
	}
	for _, r := range s.Repos {
		flag := ""
		if _, err := os.Stat(r); err != nil {
			flag = " [missing]"
		}
		fmt.Printf("  - %s%s\n", r, flag)
	}
	fmt.Println()
	fmt.Println("OS scheduler:")
	switch runtime.GOOS {
	case "darwin":
		path := launchdPlistPath()
		if _, err := os.Stat(path); err == nil {
			fmt.Println("  launchd plist installed at", path)
		} else {
			fmt.Println("  not installed (run `gitmate schedule install`)")
		}
	case "linux":
		path := systemdUnitPath("gitmate.timer")
		if _, err := os.Stat(path); err == nil {
			fmt.Println("  systemd timer installed at", path)
		} else {
			fmt.Println("  not installed (run `gitmate schedule install`)")
		}
	default:
		fmt.Println("  unsupported OS for auto-install — see `gitmate schedule install --print`")
	}
	return nil
}

var scheduleSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set schedule fields (use --time, --enable, --disable, or other flags)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.GlobalPath()
		data, err := loadConfigMap(path)
		if err != nil {
			return err
		}
		sched, _ := data["schedule"].(map[string]any)
		if sched == nil {
			sched = map[string]any{}
			data["schedule"] = sched
		}
		changed := false
		timeChanged := false
		if scheduleTime != "" {
			if _, _, err := parseHHMM(scheduleTime); err != nil {
				return err
			}
			sched["time"] = scheduleTime
			changed = true
			timeChanged = true
		}
		if scheduleEnable {
			sched["enabled"] = true
			changed = true
		}
		if scheduleDisable {
			sched["enabled"] = false
			changed = true
		}
		if !changed {
			return fmt.Errorf("nothing to set — pass --time, --enable, or --disable")
		}
		if err := writeConfigMap(path, data); err != nil {
			return err
		}
		fmt.Println("✓ schedule updated in", path)

		enabled, _ := sched["enabled"].(bool)
		switch {
		case scheduleDisable:
			if osSchedulerInstalled() {
				fmt.Println("→ disabling OS scheduler...")
				if err := osSchedulerUninstall(); err != nil {
					fmt.Println("uninstall warning:", err)
				}
			}
		case enabled:
			needInstall := !osSchedulerInstalled() || timeChanged
			if needInstall {
				fmt.Println("→ wiring OS scheduler...")
				if err := osSchedulerInstall(); err != nil {
					fmt.Println("install failed:", err)
					fmt.Println("(run `gitmate schedule install` manually to retry)")
				}
			}
		}
		return nil
	},
}

var scheduleAddRepoCmd = &cobra.Command{
	Use:   "add-repo <path>",
	Short: "Add a repo to the scheduled sync list",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]
		if !filepath.IsAbs(repo) {
			abs, err := filepath.Abs(repo)
			if err != nil {
				return err
			}
			repo = abs
		}
		if root, err := tools.RepoRoot(contextWithDir(repo)); err == nil && root != "" {
			repo = root
		}
		if err := mutateScheduleRepos(func(list []string) []string {
			for _, r := range list {
				if r == repo {
					fmt.Println("(already present)")
					return list
				}
			}
			fmt.Println("✓ added", repo)
			return append(list, repo)
		}); err != nil {
			return err
		}
		app, _ := newApp()
		if app != nil && app.Cfg.Schedule.Enabled && !osSchedulerInstalled() {
			fmt.Println("→ wiring OS scheduler...")
			if err := osSchedulerInstall(); err != nil {
				fmt.Println("install failed:", err)
			}
		}
		return nil
	},
}

var scheduleRemoveRepoCmd = &cobra.Command{
	Use:   "remove-repo <path>",
	Short: "Remove a repo from the scheduled sync list",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]
		if !filepath.IsAbs(repo) {
			abs, err := filepath.Abs(repo)
			if err == nil {
				repo = abs
			}
		}
		return mutateScheduleRepos(func(list []string) []string {
			out := list[:0]
			for _, r := range list {
				if r != repo {
					out = append(out, r)
				}
			}
			fmt.Println("✓ removed", repo)
			return out
		})
	},
}

var scheduleRunNowCmd = &cobra.Command{
	Use:   "run-now",
	Short: "Run scheduled sync now (sync --auto --all across configured repos)",
	RunE: func(cmd *cobra.Command, args []string) error {
		selfPath, err := os.Executable()
		if err != nil {
			return err
		}
		c := exec.Command(selfPath, "sync", "--auto", "--all")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Env = os.Environ()
		return c.Run()
	},
}

var scheduleInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install OS-level scheduler (launchd on macOS, systemd user timer on Linux)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if schedulePrint {
			return osSchedulerPrint()
		}
		return osSchedulerInstall()
	},
}

var scheduleUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall OS scheduler entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		return osSchedulerUninstall()
	},
}

func osSchedulerInstalled() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(launchdPlistPath())
		return err == nil
	case "linux":
		_, err := os.Stat(systemdUnitPath("gitmate.timer"))
		return err == nil
	}
	return false
}

func osSchedulerInstall() error {
	app, err := newApp()
	if err != nil {
		return err
	}
	hh, mm, err := parseHHMM(app.Cfg.Schedule.Time)
	if err != nil {
		return fmt.Errorf("config schedule.time invalid: %w", err)
	}
	selfPath, err := os.Executable()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		plist := renderLaunchdPlist(selfPath, hh, mm)
		path := launchdPlistPath()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "unload", path).Run()
		if out, err := exec.Command("launchctl", "load", path).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl load: %w: %s", err, string(out))
		}
		fmt.Println("✓ launchd plist installed and loaded:", path)
		return nil
	case "linux":
		service := renderSystemdService(selfPath)
		timer := renderSystemdTimer(hh, mm)
		servicePath := systemdUnitPath("gitmate.service")
		timerPath := systemdUnitPath("gitmate.timer")
		if err := os.MkdirAll(filepath.Dir(servicePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(servicePath, []byte(service), 0644); err != nil {
			return err
		}
		if err := os.WriteFile(timerPath, []byte(timer), 0644); err != nil {
			return err
		}
		for _, args := range [][]string{
			{"--user", "daemon-reload"},
			{"--user", "enable", "--now", "gitmate.timer"},
		} {
			if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl %v: %w: %s", args, err, string(out))
			}
		}
		fmt.Println("✓ systemd timer installed:", timerPath)
		return nil
	}
	return fmt.Errorf("unsupported OS %q — use --print and install manually", runtime.GOOS)
}

func osSchedulerUninstall() error {
	switch runtime.GOOS {
	case "darwin":
		path := launchdPlistPath()
		_ = exec.Command("launchctl", "unload", path).Run()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Println("✓ launchd plist removed:", path)
		return nil
	case "linux":
		_ = exec.Command("systemctl", "--user", "disable", "--now", "gitmate.timer").Run()
		for _, name := range []string{"gitmate.timer", "gitmate.service"} {
			p := systemdUnitPath(name)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		fmt.Println("✓ systemd units removed")
		return nil
	}
	return fmt.Errorf("unsupported OS %q", runtime.GOOS)
}

func osSchedulerPrint() error {
	app, err := newApp()
	if err != nil {
		return err
	}
	hh, mm, err := parseHHMM(app.Cfg.Schedule.Time)
	if err != nil {
		return err
	}
	selfPath, err := os.Executable()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		fmt.Println(renderLaunchdPlist(selfPath, hh, mm))
	case "linux":
		fmt.Println("# gitmate.service")
		fmt.Println(renderSystemdService(selfPath))
		fmt.Println("# gitmate.timer")
		fmt.Println(renderSystemdTimer(hh, mm))
	default:
		return fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
	return nil
}

func mutateScheduleRepos(fn func([]string) []string) error {
	path := config.GlobalPath()
	data, err := loadConfigMap(path)
	if err != nil {
		return err
	}
	sched, _ := data["schedule"].(map[string]any)
	if sched == nil {
		sched = map[string]any{}
		data["schedule"] = sched
	}
	var list []string
	if raw, ok := sched["repos"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				list = append(list, s)
			}
		}
	}
	list = fn(list)
	asAny := make([]any, len(list))
	for i, v := range list {
		asAny[i] = v
	}
	sched["repos"] = asAny
	return writeConfigMap(path, data)
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected HH:MM, got %q", s)
	}
	hh, err := strconv.Atoi(parts[0])
	if err != nil || hh < 0 || hh > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", s)
	}
	mm, err := strconv.Atoi(parts[1])
	if err != nil || mm < 0 || mm > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", s)
	}
	return hh, mm, nil
}

func contextWithDir(_ string) context.Context { return context.Background() }

func launchdPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.gitmate.daemon.plist")
}

func systemdUnitPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", name)
}

func renderLaunchdPlist(binary string, hh, mm int) string {
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".gitmate", "schedule.log")
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.gitmate.daemon</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>sync</string>
    <string>--auto</string>
    <string>--all</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key><integer>%d</integer>
    <key>Minute</key><integer>%d</integer>
  </dict>
  <key>RunAtLoad</key>
  <false/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, binary, hh, mm, logPath, logPath)
}

func renderSystemdService(binary string) string {
	return fmt.Sprintf(`[Unit]
Description=gitmate scheduled sync

[Service]
Type=oneshot
ExecStart=%s sync --auto --all
`, binary)
}

func renderSystemdTimer(hh, mm int) string {
	return fmt.Sprintf(`[Unit]
Description=gitmate daily sync timer

[Timer]
OnCalendar=*-*-* %02d:%02d:00
Persistent=true
Unit=gitmate.service

[Install]
WantedBy=timers.target
`, hh, mm)
}

func init() {
	scheduleSetCmd.Flags().StringVar(&scheduleTime, "time", "", "daily fire time HH:MM")
	scheduleSetCmd.Flags().BoolVar(&scheduleEnable, "enable", false, "enable schedule")
	scheduleSetCmd.Flags().BoolVar(&scheduleDisable, "disable", false, "disable schedule")
	scheduleInstallCmd.Flags().BoolVar(&schedulePrint, "print", false, "print service file without installing")
	scheduleCmd.AddCommand(scheduleSetCmd, scheduleAddRepoCmd, scheduleRemoveRepoCmd, scheduleRunNowCmd, scheduleInstallCmd, scheduleUninstallCmd)
}
