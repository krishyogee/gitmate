package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorAccent  = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	ColorMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	ColorOK      = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	ColorWarn    = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}
	ColorErr     = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	ColorBorder  = lipgloss.AdaptiveColor{Light: "#A78BFA", Dark: "#7C3AED"}
	ColorCommand = lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#38BDF8"}
)

var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		MarginRight(1)

	Subtle = lipgloss.NewStyle().
		Foreground(ColorMuted)

	OK = lipgloss.NewStyle().
		Foreground(ColorOK).
		Bold(true)

	Warn = lipgloss.NewStyle().
		Foreground(ColorWarn).
		Bold(true)

	Err = lipgloss.NewStyle().
		Foreground(ColorErr).
		Bold(true)

	Cmd = lipgloss.NewStyle().
		Foreground(ColorCommand).
		Bold(true)

	Card = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 2).
		Width(64)

	CardTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		MarginBottom(1)

	KV = lipgloss.NewStyle().
		Foreground(ColorMuted)

	KVValue = lipgloss.NewStyle().
		Bold(true)

	Hint = lipgloss.NewStyle().
		Foreground(ColorMuted).
		Italic(true)

	RiskLow    = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	RiskMed    = lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)
	RiskHigh   = lipgloss.NewStyle().Foreground(ColorErr).Bold(true)
	Highlight  = lipgloss.NewStyle().Reverse(true).Padding(0, 1)
	MenuItem   = lipgloss.NewStyle().Padding(0, 2)
	MenuActive = lipgloss.NewStyle().Padding(0, 2).Foreground(ColorAccent).Bold(true)
)

func RiskColor(level string) lipgloss.Style {
	switch level {
	case "HIGH":
		return RiskHigh
	case "MEDIUM":
		return RiskMed
	default:
		return RiskLow
	}
}
