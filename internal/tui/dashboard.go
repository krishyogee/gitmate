package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DashboardData struct {
	RepoRoot     string
	Branch       string
	Base         string
	Ahead        int
	Behind       int
	ChangedFiles int
	OverlapCount int
	RiskLevel    string
	HasAIKey     bool
	NotInRepo    bool
	Version      string
}

type Action struct {
	Key     string
	Command string
	Title   string
	Desc    string
}

var actions = []Action{
	{"s", "ship", "ship", "Generate commit message + commit + optional PR"},
	{"y", "sync", "sync", "Fetch + integrate origin/<branch> + base"},
	{"r", "resolve", "resolve <file>", "Explain + resolve conflicts"},
	{"c", "check", "check", "Predict merge pain (overlap, hotspots)"},
	{"t", "status", "status", "Branch state + risk indicator"},
	{"x", "explain", "explain", "Explain a diff in plain language"},
	{"p", "push", "push", "Push current branch to origin"},
	{"m", "metrics", "metrics", "Approval rate + latency + scores"},
	{"i", "init", "init", "Configure provider + API key"},
	{"f", "config", "config", "Show effective config"},
}

type dashboardModel struct {
	data     DashboardData
	cursor   int
	selected string
	width    int
	height   int
	quit     bool
}

func RunDashboard(data DashboardData) (string, error) {
	p := tea.NewProgram(dashboardModel{data: data}, tea.WithAltScreen())
	out, err := p.Run()
	if err != nil {
		return "", err
	}
	m := out.(dashboardModel)
	return m.selected, nil
}

func (m dashboardModel) Init() tea.Cmd { return nil }

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(actions)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = actions[m.cursor].Command
			return m, tea.Quit
		default:
			for _, a := range actions {
				if msg.String() == a.Key {
					m.selected = a.Command
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m dashboardModel) View() string {
	if m.quit {
		return ""
	}

	header := Title.Render("gitmate") + " " + Subtle.Render(m.data.Version)

	var status string
	if m.data.NotInRepo {
		status = Warn.Render("not in a git repo") + " " + Subtle.Render("— some actions need a repo")
	} else {
		risk := RiskColor(m.data.RiskLevel).Render(m.data.RiskLevel)
		status = strings.Join([]string{
			fmt.Sprintf("%s %s", KV.Render("repo"), KVValue.Render(shorten(m.data.RepoRoot, 50))),
			fmt.Sprintf("%s %s %s", KV.Render("branch"), KVValue.Render(m.data.Branch),
				Subtle.Render(fmt.Sprintf("(↑%d ↓%d)", m.data.Ahead, m.data.Behind))),
			fmt.Sprintf("%s %s", KV.Render("base"), KVValue.Render(m.data.Base)),
			fmt.Sprintf("%s %s   %s %d", KV.Render("risk"), risk, KV.Render("overlap"), m.data.OverlapCount),
		}, "\n")
	}

	keyHint := ""
	if !m.data.HasAIKey {
		keyHint = "\n" + Warn.Render("⚠ no AI key in env") + " " + Subtle.Render("— run `gitmate init` or set ANTHROPIC_API_KEY")
	}

	statusBox := Card.Width(64).Render(status + keyHint)

	var menu strings.Builder
	menu.WriteString(CardTitle.Render("actions") + "\n")
	for i, a := range actions {
		key := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render("[" + a.Key + "]")
		title := a.Title
		desc := Subtle.Render("· " + a.Desc)
		row := fmt.Sprintf("%s  %s  %s", key, title, desc)
		if i == m.cursor {
			row = MenuActive.Render("▸ " + row)
		} else {
			row = MenuItem.Render("  " + row)
		}
		menu.WriteString(row + "\n")
	}
	menuBox := Card.Width(64).Render(menu.String())

	hint := Hint.Render("↑/↓ navigate  ·  enter select  ·  type letter shortcut  ·  q quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		header,
		"",
		statusBox,
		"",
		menuBox,
		"",
		hint,
	)
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}
