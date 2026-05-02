package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Cap on stored checkpoints per repo. Older entries pruned.
const Cap = 50

// FileBackup records a single file backed up before overwrite.
type FileBackup struct {
	Path       string `json:"path"`
	BackupPath string `json:"backup_path"`
}

// Op is a single recorded mutation. Status: pending|done|failed|undone.
// OpType: commit|rebase|merge|stash|push|pr_create|file_write.
type Op struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	Command         string            `json:"command"`
	OpType          string            `json:"op_type"`
	Branch          string            `json:"branch,omitempty"`
	HeadBefore      string            `json:"head_before,omitempty"`
	HeadAfter       string            `json:"head_after,omitempty"`
	BackupRef       string            `json:"backup_ref,omitempty"`
	StashRef        string            `json:"stash_ref,omitempty"`
	Remote          string            `json:"remote,omitempty"`
	RemoteBranch    string            `json:"remote_branch,omitempty"`
	RemoteSHABefore string            `json:"remote_sha_before,omitempty"`
	RemoteSHAAfter  string            `json:"remote_sha_after,omitempty"`
	PRNumber        string            `json:"pr_number,omitempty"`
	PRURL           string            `json:"pr_url,omitempty"`
	FilesWritten    []FileBackup      `json:"files_written,omitempty"`
	Args            map[string]string `json:"args,omitempty"`
	Reversible      bool              `json:"reversible"`
	ReasonIfNot     string            `json:"reason_if_not,omitempty"`
	Status          string            `json:"status"`
	Note            string            `json:"note,omitempty"`
}

// Store persists Ops as JSON in <repo>/.gitmate/checkpoints.json.
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(repoRoot string) *Store {
	return &Store{path: filepath.Join(repoRoot, ".gitmate", "checkpoints.json")}
}

func (s *Store) Path() string { return s.path }

func (s *Store) load() ([]Op, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ops []Op
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &ops); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	return ops, nil
}

func (s *Store) save(ops []Op) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	if len(ops) > Cap {
		ops = ops[len(ops)-Cap:]
	}
	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// Append adds op to store. Caller fills fields beforehand.
func (s *Store) Append(op Op) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops, err := s.load()
	if err != nil {
		return err
	}
	ops = append(ops, op)
	return s.save(ops)
}

// Update overwrites op with matching ID.
func (s *Store) Update(op Op) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops, err := s.load()
	if err != nil {
		return err
	}
	for i := range ops {
		if ops[i].ID == op.ID {
			ops[i] = op
			return s.save(ops)
		}
	}
	return fmt.Errorf("op %s not found", op.ID)
}

// List returns ops newest-first.
func (s *Store) List() ([]Op, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(ops, func(i, j int) bool {
		return ops[i].Timestamp.After(ops[j].Timestamp)
	})
	return ops, nil
}

// Get fetches op by id.
func (s *Store) Get(id string) (*Op, error) {
	ops, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range ops {
		if ops[i].ID == id {
			return &ops[i], nil
		}
	}
	return nil, fmt.Errorf("checkpoint %s not found", id)
}

// LastUndoable returns most recent op that can still be undone.
func (s *Store) LastUndoable() (*Op, error) {
	ops, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range ops {
		if ops[i].Status == "done" && ops[i].Reversible {
			return &ops[i], nil
		}
	}
	return nil, nil
}
