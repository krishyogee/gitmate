package agent

import (
	"math"
	"regexp"
	"strings"
)

type Evaluator interface {
	Score(action string, output string) float64
	IsValid(action string, output string) bool
	PassThreshold() float64
	FailThreshold() float64
}

type CommitEvaluator struct{}

var conventionalRe = regexp.MustCompile(`^(feat|fix|chore|docs|refactor|test|style|perf|ci|build|revert)(\(.+\))?: .+`)

func (e CommitEvaluator) Score(action, output string) float64 {
	if action != "generate_commit" && action != "refine_commit" {
		return 1.0
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return 0
	}
	score := 1.0
	lines := strings.Split(output, "\n")

	if len(lines[0]) > 72 {
		score -= 0.3
	}
	if !conventionalRe.MatchString(lines[0]) {
		score -= 0.2
	}
	lower := strings.ToLower(output)
	for _, g := range []string{"update", "fix bug", "changes", "wip", "misc", "stuff"} {
		if strings.Contains(lower, " "+g+" ") || strings.HasSuffix(lower, " "+g) || strings.HasSuffix(lower, ": "+g) {
			score -= 0.2
			break
		}
	}
	if len(lines) < 3 && len(lines[0]) < 30 {
		score -= 0.1
	}
	return math.Max(score, 0)
}

func (e CommitEvaluator) IsValid(action, output string) bool {
	return strings.TrimSpace(output) != ""
}

func (e CommitEvaluator) PassThreshold() float64 { return 0.8 }
func (e CommitEvaluator) FailThreshold() float64 { return 0.4 }

type ConflictEvaluator struct{}

func (e ConflictEvaluator) Score(action, output string) float64 {
	if action != "resolve_conflict" {
		return 1.0
	}
	if strings.Contains(output, "<<<<<<<") || strings.Contains(output, ">>>>>>>") || strings.Contains(output, "=======\n") {
		return 0
	}
	if strings.TrimSpace(output) == "" {
		return 0
	}
	return 0.85
}

func (e ConflictEvaluator) IsValid(action, output string) bool {
	if action != "resolve_conflict" {
		return true
	}
	return !strings.Contains(output, "<<<<<<<") && !strings.Contains(output, ">>>>>>>")
}

func (e ConflictEvaluator) PassThreshold() float64 { return 0.8 }
func (e ConflictEvaluator) FailThreshold() float64 { return 0.4 }

type ExplainEvaluator struct{}

func (e ExplainEvaluator) Score(action, output string) float64 {
	if strings.TrimSpace(output) == "" {
		return 0
	}
	if len(output) < 30 {
		return 0.5
	}
	return 0.9
}

func (e ExplainEvaluator) IsValid(_, output string) bool { return strings.TrimSpace(output) != "" }
func (e ExplainEvaluator) PassThreshold() float64        { return 0.8 }
func (e ExplainEvaluator) FailThreshold() float64        { return 0.3 }

type RiskEvaluator struct{}

func (e RiskEvaluator) Score(_, output string) float64 {
	if strings.TrimSpace(output) == "" {
		return 0
	}
	return 0.9
}
func (e RiskEvaluator) IsValid(_, output string) bool { return strings.TrimSpace(output) != "" }
func (e RiskEvaluator) PassThreshold() float64        { return 0.8 }
func (e RiskEvaluator) FailThreshold() float64        { return 0.3 }
