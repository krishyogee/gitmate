package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type ApprovalView struct {
	Action      string
	Risk        string
	Description string
	Input       string
	Preview     string
}

func RenderApprovalCard(v ApprovalView) string {
	riskStyle := lipgloss.NewStyle().Bold(true)
	switch v.Risk {
	case "READ":
		riskStyle = riskStyle.Foreground(ColorOK)
	case "ADVISE":
		riskStyle = riskStyle.Foreground(ColorCommand)
	case "PROPOSE":
		riskStyle = riskStyle.Foreground(ColorWarn)
	case "EXECUTE":
		riskStyle = riskStyle.Foreground(ColorErr)
	}

	header := CardTitle.Render("gitmate · action required")

	rows := []string{
		fmt.Sprintf("%s  %s", KV.Render("action"), Cmd.Render(v.Action)),
		fmt.Sprintf("%s    %s", KV.Render("risk"), riskStyle.Render(v.Risk)),
	}
	if v.Description != "" {
		rows = append(rows, fmt.Sprintf("%s     %s", KV.Render("why"), KVValue.Render(truncate(v.Description, 50))))
	}

	body := strings.Join(rows, "\n")
	card := Card.Render(header + "\n" + body)

	out := card
	if v.Input != "" {
		out += "\n" + Subtle.Render("─── input ───")
		out += "\n" + indent(truncate(v.Input, 800), "  ")
		out += "\n" + Subtle.Render("─────────────")
	}
	return out
}

func RenderApprovalPrompt() string {
	keys := []string{
		Cmd.Render("y") + " yes",
		Cmd.Render("a") + " allow session",
		Cmd.Render("p") + " preview",
		Cmd.Render("e") + " edit",
		Cmd.Render("n") + " no",
		Cmd.Render("?") + " explain",
	}
	return strings.Join(keys, "  ") + "  " + Hint.Render("›")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
