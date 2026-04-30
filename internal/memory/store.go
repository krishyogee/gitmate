package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LongTerm struct {
	RepoPatterns    map[string]RepoProfile `json:"repo_patterns"`
	UserStyle       UserPreferences        `json:"user_style"`
	ConflictHistory []ConflictRecord       `json:"conflict_history"`
}

type RepoProfile struct {
	CommitStyle string    `json:"commit_style"`
	HotFiles    []string  `json:"hot_files"`
	TestCommand string    `json:"test_command"`
	LastSync    time.Time `json:"last_sync"`
	Approvals   int       `json:"approvals"`
	Rejections  int       `json:"rejections"`
	Edits       int       `json:"edits"`
}

type UserPreferences struct {
	PrefersShortCommits bool   `json:"prefers_short_commits"`
	AlwaysRunTests      bool   `json:"always_run_tests"`
	DefaultApprovalMode string `json:"default_approval_mode"`
}

type ConflictRecord struct {
	FilePattern  string    `json:"file_pattern"`
	Resolution   string    `json:"resolution"`
	UserAccepted bool      `json:"user_accepted"`
	Timestamp    time.Time `json:"timestamp"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data *LongTerm
}

func NewStore() *Store {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".gitmate")
	_ = os.MkdirAll(dir, 0755)
	s := &Store{path: filepath.Join(dir, "memory.json")}
	s.load()
	return s
}

func (s *Store) load() {
	s.data = &LongTerm{RepoPatterns: map[string]RepoProfile{}}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, s.data)
	if s.data.RepoPatterns == nil {
		s.data.RepoPatterns = map[string]RepoProfile{}
	}
}

func (s *Store) save() {
	buf, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, buf, 0644)
}

func (s *Store) RepoProfile(repoRoot string) RepoProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.RepoPatterns[repoRoot]
}

func (s *Store) UserStyle() UserPreferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.UserStyle
}

func (s *Store) UpdateRepoProfile(repoRoot string, fn func(*RepoProfile)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := s.data.RepoPatterns[repoRoot]
	fn(&rp)
	s.data.RepoPatterns[repoRoot] = rp
	s.save()
}

func (s *Store) UpdateUserStyle(fn func(*UserPreferences)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data.UserStyle)
	s.save()
}

func (s *Store) RecordConflict(rec ConflictRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ConflictHistory = append(s.data.ConflictHistory, rec)
	if len(s.data.ConflictHistory) > 200 {
		s.data.ConflictHistory = s.data.ConflictHistory[len(s.data.ConflictHistory)-200:]
	}
	s.save()
}

func (s *Store) PersistSession(repoRoot string, session *Session) {
	s.UpdateRepoProfile(repoRoot, func(rp *RepoProfile) {
		for _, a := range session.Attempts {
			switch a.UserAction {
			case "approved", "session", "auto":
				rp.Approvals++
			case "rejected":
				rp.Rejections++
			case "edited":
				rp.Edits++
			}
		}
	})
}

func (s *Store) RepoContext(repoRoot string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := s.data.RepoPatterns[repoRoot]
	us := s.data.UserStyle
	var b strings.Builder
	if rp.CommitStyle != "" {
		fmt.Fprintf(&b, "commit_style=%s ", rp.CommitStyle)
	}
	if rp.TestCommand != "" {
		fmt.Fprintf(&b, "test_command=%q ", rp.TestCommand)
	}
	if len(rp.HotFiles) > 0 {
		fmt.Fprintf(&b, "hot_files=%v ", rp.HotFiles)
	}
	if us.PrefersShortCommits {
		b.WriteString("prefers_short_commits=true ")
	}
	fmt.Fprintf(&b, "approvals=%d rejections=%d edits=%d", rp.Approvals, rp.Rejections, rp.Edits)
	return b.String()
}
