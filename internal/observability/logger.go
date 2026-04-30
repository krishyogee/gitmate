package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Command      string    `json:"command,omitempty"`
	Task         string    `json:"task,omitempty"`
	Action       string    `json:"action,omitempty"`
	Model        string    `json:"model,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	LatencyMs    int64     `json:"latency_ms,omitempty"`
	Score        float64   `json:"score,omitempty"`
	UserAction   string    `json:"user_action,omitempty"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
	Note         string    `json:"note,omitempty"`
}

type Logger struct {
	path string
	mu   sync.Mutex
}

func NewLogger() *Logger {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".gitmate")
	_ = os.MkdirAll(dir, 0755)
	return &Logger{path: filepath.Join(dir, "ai-log.jsonl")}
}

func (l *Logger) write(e LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(e)
	fmt.Fprintln(f, string(data))
}

func (l *Logger) LogStep(task, action, _ string, score float64) {
	l.write(LogEntry{Task: task, Action: action, Score: score, Success: score > 0})
}

func (l *Logger) LogAICall(provider, model, task string, in, out int, latencyMs int64, err error) {
	e := LogEntry{Provider: provider, Model: model, Task: task,
		InputTokens: in, OutputTokens: out, LatencyMs: latencyMs, Success: err == nil}
	if err != nil {
		e.Error = err.Error()
	}
	l.write(e)
}

func (l *Logger) LogApproval(action, userAction string) {
	l.write(LogEntry{Action: action, UserAction: userAction, Success: userAction == "approved" || userAction == "session"})
}

func (l *Logger) LogCommand(command, note string, success bool, err error) {
	e := LogEntry{Command: command, Note: note, Success: success}
	if err != nil {
		e.Error = err.Error()
	}
	l.write(e)
}

func (l *Logger) Path() string { return l.path }
