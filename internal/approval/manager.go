package approval

import (
	"fmt"

	"github.com/krishyogee/gitmate/internal/observability"
)

type RiskLevel int

const (
	READ RiskLevel = iota
	ADVISE
	PROPOSE
	EXECUTE
)

func (r RiskLevel) String() string {
	switch r {
	case READ:
		return "READ"
	case ADVISE:
		return "ADVISE"
	case PROPOSE:
		return "PROPOSE"
	case EXECUTE:
		return "EXECUTE"
	}
	return "UNKNOWN"
}

type Decision string

const (
	DecisionYes      Decision = "yes"
	DecisionSession  Decision = "session"
	DecisionNo       Decision = "no"
	DecisionEdit     Decision = "edit"
	DecisionPreview  Decision = "preview"
)

type Card struct {
	Action      string
	Input       string
	Risk        RiskLevel
	Description string
	Preview     string
}

type Manager struct {
	allow  map[string]bool
	logger *observability.Logger
	ui     UI
	auto   bool
}

func NewManager(logger *observability.Logger) *Manager {
	return &Manager{
		allow:  map[string]bool{},
		logger: logger,
		ui:     &TerminalUI{},
	}
}

func (m *Manager) SetAuto(v bool) { m.auto = v }
func (m *Manager) SetUI(u UI)     { m.ui = u }

var actionRisk = map[string]RiskLevel{
	"git_diff":         READ,
	"git_status":       READ,
	"git_log":          READ,
	"parse_conflicts":  READ,
	"fetch_hotspots":   READ,
	"generate_commit":  ADVISE,
	"refine_commit":    ADVISE,
	"explain_conflict": ADVISE,
	"explain_diff":     ADVISE,
	"create_pr":        PROPOSE,
	"resolve_conflict": PROPOSE,
	"git_commit":       EXECUTE,
	"git_push":         EXECUTE,
	"git_apply":        EXECUTE,
	"run_tests":        EXECUTE,
	"run_lint":         EXECUTE,
	"write_file":       EXECUTE,
}

func (m *Manager) RiskOf(action string) RiskLevel {
	if r, ok := actionRisk[action]; ok {
		return r
	}
	return EXECUTE
}

func (m *Manager) IsRequired(action string) bool {
	if m.auto {
		return false
	}
	r := m.RiskOf(action)
	switch r {
	case READ:
		return false
	case ADVISE:
		return !m.allow[fmt.Sprintf("ADVISE:%s", action)]
	case PROPOSE, EXECUTE:
		return true
	}
	return true
}

func (m *Manager) Request(card Card) (Decision, string, error) {
	if m.auto {
		m.logger.LogApproval(card.Action, "auto")
		return DecisionYes, card.Input, nil
	}
	card.Risk = m.RiskOf(card.Action)
	dec, edited, err := m.ui.Prompt(card)
	if err != nil {
		m.logger.LogApproval(card.Action, "error")
		return DecisionNo, "", err
	}
	switch dec {
	case DecisionSession:
		m.allow[fmt.Sprintf("%s:%s", card.Risk.String(), card.Action)] = true
		m.logger.LogApproval(card.Action, "session")
	case DecisionYes:
		m.logger.LogApproval(card.Action, "approved")
	case DecisionNo:
		m.logger.LogApproval(card.Action, "rejected")
	case DecisionEdit:
		m.logger.LogApproval(card.Action, "edited")
	}
	return dec, edited, nil
}
