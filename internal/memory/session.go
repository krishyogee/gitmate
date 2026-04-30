package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Session struct {
	Task       string
	Attempts   []Attempt
	LastOutput string
	RepoRoot   string
	StartTime  time.Time
	repoCtx    string
}

type Attempt struct {
	Action     string  `json:"action"`
	Input      string  `json:"input,omitempty"`
	Output     string  `json:"output"`
	Score      float64 `json:"score"`
	UserAction string  `json:"user_action,omitempty"`
}

func NewSession(repoRoot string) *Session {
	return &Session{
		RepoRoot:  repoRoot,
		StartTime: time.Now(),
	}
}

func (s *Session) SetRepoContext(c string) { s.repoCtx = c }
func (s *Session) RepoContext() string     { return s.repoCtx }

func (s *Session) Update(action, result string) {
	s.Attempts = append(s.Attempts, Attempt{Action: action, Output: result})
	s.LastOutput = result
}

func (s *Session) UpdateScore(score float64) {
	if len(s.Attempts) > 0 {
		s.Attempts[len(s.Attempts)-1].Score = score
	}
}

func (s *Session) UpdateUserAction(ua string) {
	if len(s.Attempts) > 0 {
		s.Attempts[len(s.Attempts)-1].UserAction = ua
	}
}

func (s *Session) StepsJSON() string {
	if len(s.Attempts) == 0 {
		return "[]"
	}
	view := s.Attempts
	if len(view) > 5 {
		view = view[len(view)-5:]
	}
	out := make([]Attempt, len(view))
	for i, a := range view {
		out[i] = a
		if len(out[i].Output) > 800 {
			out[i].Output = out[i].Output[:800] + "...(truncated)"
		}
	}
	buf, _ := json.Marshal(out)
	return string(buf)
}

func (s *Session) Summary() string {
	var b strings.Builder
	for _, a := range s.Attempts {
		fmt.Fprintf(&b, "- %s (score=%.2f)\n", a.Action, a.Score)
	}
	return b.String()
}
