package checkpoint

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/krishyogee/gitmate/internal/tools"
)

// Recorder wraps Store with helpers to capture git state and create backups.
type Recorder struct {
	Store    *Store
	RepoRoot string
}

func NewRecorder(repoRoot string) *Recorder {
	if repoRoot == "" {
		return nil
	}
	return &Recorder{Store: NewStore(repoRoot), RepoRoot: repoRoot}
}

func newID() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(buf[:])
}

// Begin creates an Op with a fresh ID, captures HEAD + branch, and returns it.
// Caller fills extra fields then calls Commit/Fail.
func (r *Recorder) Begin(ctx context.Context, command, opType string) *Op {
	if r == nil {
		return nil
	}
	op := &Op{
		ID:        newID(),
		Timestamp: time.Now().UTC(),
		Command:   command,
		OpType:    opType,
		Status:    "pending",
		Reversible: true,
	}
	if branch, err := tools.CurrentBranch(ctx); err == nil {
		op.Branch = branch
	}
	if sha, err := captureHead(ctx); err == nil {
		op.HeadBefore = sha
	}
	return op
}

// CreateBackupRef sets refs/gitmate/backup/<id> to current HEAD so destructive
// rebase/merge can be reset later. Best-effort; failure does not block op.
func (r *Recorder) CreateBackupRef(ctx context.Context, op *Op) {
	if op == nil || op.HeadBefore == "" {
		return
	}
	ref := fmt.Sprintf("refs/gitmate/backup/%s", op.ID)
	if _, err := tools.RunGit(ctx, "update-ref", ref, op.HeadBefore); err == nil {
		op.BackupRef = ref
	}
}

// CaptureRemoteSha records the remote tip so an undo can force-with-lease back.
func (r *Recorder) CaptureRemoteSha(ctx context.Context, op *Op, remote, branch string) {
	if op == nil {
		return
	}
	op.Remote = remote
	op.RemoteBranch = branch
	if sha, err := tools.RunGit(ctx, "rev-parse", remote+"/"+branch); err == nil {
		op.RemoteSHABefore = strings.TrimSpace(sha)
	}
}

// BackupFile copies file to .gitmate/backups/<id>/<flat-path> and records it.
// Used before overwriting a file (e.g., conflict resolution write).
func (r *Recorder) BackupFile(op *Op, path string) error {
	if r == nil || op == nil {
		return nil
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.RepoRoot, path)
	}
	src, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer src.Close()
	flat := strings.ReplaceAll(path, string(os.PathSeparator), "__")
	dir := filepath.Join(r.RepoRoot, ".gitmate", "backups", op.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	dstPath := filepath.Join(dir, flat)
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	op.FilesWritten = append(op.FilesWritten, FileBackup{Path: path, BackupPath: dstPath})
	return nil
}

// Commit finalizes Op as done, captures HEAD-after, and persists.
func (r *Recorder) Commit(ctx context.Context, op *Op) error {
	if r == nil || op == nil {
		return nil
	}
	if sha, err := captureHead(ctx); err == nil {
		op.HeadAfter = sha
	}
	op.Status = "done"
	return r.Store.Append(*op)
}

// Fail marks Op failed and persists. Cleans up backup ref since op did not land.
func (r *Recorder) Fail(ctx context.Context, op *Op, errMsg string) {
	if r == nil || op == nil {
		return
	}
	op.Status = "failed"
	op.Note = errMsg
	if op.BackupRef != "" {
		_, _ = tools.RunGit(ctx, "update-ref", "-d", op.BackupRef)
		op.BackupRef = ""
	}
	_ = r.Store.Append(*op)
}

// MarkIrreversible sets reversible=false with reason.
func (r *Recorder) MarkIrreversible(op *Op, reason string) {
	if op == nil {
		return
	}
	op.Reversible = false
	op.ReasonIfNot = reason
}

func captureHead(ctx context.Context) (string, error) {
	out, err := tools.RunGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LatestStashRef returns sha of stash@{0} (the just-pushed stash).
func LatestStashRef(ctx context.Context) string {
	out, err := tools.RunGit(ctx, "rev-parse", "stash@{0}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
