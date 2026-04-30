package conflict

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	content := `package x

func A() int {
<<<<<<< HEAD
	return 1
=======
	return 2
>>>>>>> branch
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	blocks, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	b := blocks[0]
	if len(b.OursLines) != 1 || b.OursLines[0] != "\treturn 1" {
		t.Errorf("ours: %v", b.OursLines)
	}
	if len(b.TheirsLines) != 1 || b.TheirsLines[0] != "\treturn 2" {
		t.Errorf("theirs: %v", b.TheirsLines)
	}
	if b.Language != "go" {
		t.Errorf("lang: %s", b.Language)
	}
}

func TestClassifyTrivialDuplicate(t *testing.T) {
	b := Block{
		FilePath:    "x.go",
		OursLines:   []string{"x := 1"},
		TheirsLines: []string{"x := 1"},
	}
	if c := Classify(b); c != Trivial {
		t.Errorf("expected trivial, got %s", c)
	}
}

func TestClassifyComplexAuth(t *testing.T) {
	b := Block{
		FilePath:    "auth/middleware.go",
		OursLines:   []string{"validateToken(req)"},
		TheirsLines: []string{"if err := jwt.Verify(token); err != nil { return err }"},
	}
	if c := Classify(b); c != Complex {
		t.Errorf("expected complex, got %s", c)
	}
}

func TestClassifyLockfile(t *testing.T) {
	b := Block{FilePath: "go.sum", OursLines: []string{"a"}, TheirsLines: []string{"b"}}
	if c := Classify(b); c != Trivial {
		t.Errorf("expected trivial for lockfile, got %s", c)
	}
}
