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
	{"s", "ship", "ship", "commit + optional PR"},
	{"y", "sync", "sync", "fetch + integrate origin + base"},
	{"c", "check", "check", "predict merge pain"},
	{"t", "status", "status", "branch + overlap + risk"},
	{"x", "explain", "explain", "explain a diff"},
	{"p", "push", "push", "push branch to origin"},
	{"m", "metrics", "metrics", "approval rate + latency"},
	{"i", "init", "init", "configure provider + key"},
	{"f", "config", "config", "show effective config"},
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
	p := tea.NewProgram(dashboardModel{data: data})
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

	var statusLine string
	if m.data.NotInRepo {
		statusLine = Warn.Render("not in a git repo")
	} else {
		risk := RiskColor(m.data.RiskLevel).Render(m.data.RiskLevel)
		statusLine = fmt.Sprintf("%s %s %s   %s %s   %s %s   %s %s",
			KV.Render("branch"), KVValue.Render(m.data.Branch),
			Subtle.Render(fmt.Sprintf("(↑%d ↓%d)", m.data.Ahead, m.data.Behind)),
			KV.Render("base"), KVValue.Render(m.data.Base),
			KV.Render("risk"), risk,
			KV.Render("overlap"), KVValue.Render(fmt.Sprintf("%d", m.data.OverlapCount)),
		)
	}

	keyHint := ""
	if !m.data.HasAIKey {
		keyHint = "  " + Warn.Render("⚠ no AI key") + " " + Subtle.Render("· run `gitmate init`")
	}

	var menu strings.Builder
	for i, a := range actions {
		key := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render("[" + a.Key + "]")
		row := fmt.Sprintf("%s  %-10s  %s", key, a.Title, Subtle.Render(a.Desc))
		if i == m.cursor {
			row = MenuActive.Render("▸ " + row)
		} else {
			row = MenuItem.Render("  " + row)
		}
		menu.WriteString(row)
		if i < len(actions)-1 {
			menu.WriteString("\n")
		}
	}

	hint := Hint.Render("↑↓/jk navigate · enter select · letter shortcut · q quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		statusLine+keyHint,
		"",
		menu.String(),
		"",
		hint,
	)
}
