package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/krishyogee/gitmate/internal/ai"
)

type Planner interface {
	Plan(ctx context.Context, task string, state *AgentState) (thought, action, input string, err error)
}

type LLMPlanner struct {
	AI            *ai.Client
	AvailableTools []string
}

type plannerResp struct {
	Thought string `json:"thought"`
	Action  string `json:"action"`
	Input   string `json:"input"`
}

func (p *LLMPlanner) Plan(ctx context.Context, task string, state *AgentState) (string, string, string, error) {
	if !p.AI.HasProvider() {
		return "", "", "", fmt.Errorf("no AI provider configured (set ANTHROPIC_API_KEY)")
	}
	user := buildPlannerPrompt(task, state, p.AvailableTools)
	system := ai.PlannerSystemPrompt
	raw, err := p.AI.Complete(ctx, system, user, "planning")
	if err != nil {
		return "", "", "", err
	}
	return parsePlannerResponse(raw)
}

func buildPlannerPrompt(task string, state *AgentState, available []string) string {
	repo := ""
	if state.Memory != nil {
		repo = state.Memory.RepoContext()
	}
	stepsJSON := "[]"
	if state.Memory != nil {
		stepsJSON = state.Memory.StepsJSON()
	}
	return fmt.Sprintf(`Task: %s
Available actions: %s
Previous steps: %s
Repo context: %s`,
		task,
		strings.Join(available, ", "),
		stepsJSON,
		repo,
	)
}

func parsePlannerResponse(raw string) (string, string, string, error) {
	raw = stripFence(raw)
	var r plannerResp
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return "", "", "", fmt.Errorf("planner returned invalid JSON: %w (raw=%s)", err, raw)
	}
	if r.Action == "" {
		return "", "", "", fmt.Errorf("planner returned empty action")
	}
	return r.Thought, r.Action, r.Input, nil
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx > 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
