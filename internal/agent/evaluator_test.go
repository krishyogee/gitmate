package agent

import "testing"

func TestCommitEvaluatorGood(t *testing.T) {
	e := CommitEvaluator{}
	score := e.Score("generate_commit", "feat(auth): add JWT verification middleware\n\nValidates tokens before request reaches handler.")
	if score < 0.8 {
		t.Errorf("expected pass, got %.2f", score)
	}
}

func TestCommitEvaluatorBad(t *testing.T) {
	e := CommitEvaluator{}
	score := e.Score("generate_commit", "update")
	if score >= 0.8 {
		t.Errorf("expected fail for generic, got %.2f", score)
	}
}

func TestCommitEvaluatorTooLong(t *testing.T) {
	e := CommitEvaluator{}
	long := "feat(auth): add JWT verification middleware that does many things and is too long for the subject line"
	score := e.Score("generate_commit", long)
	if score >= 0.8 {
		t.Errorf("expected penalty for >72 chars, got %.2f", score)
	}
}

func TestConflictEvaluatorMarkers(t *testing.T) {
	e := ConflictEvaluator{}
	if s := e.Score("resolve_conflict", "<<<<<<< HEAD\nfoo\n>>>>>>> b"); s != 0 {
		t.Errorf("expected 0 with markers, got %.2f", s)
	}
}

func TestConflictEvaluatorClean(t *testing.T) {
	e := ConflictEvaluator{}
	if s := e.Score("resolve_conflict", "func A() {}"); s < 0.8 {
		t.Errorf("expected pass, got %.2f", s)
	}
}
