package ai

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	cases := map[string]string{
		"api_key=abcdef1234567890abcdef":   "[REDACTED]",
		"password: 'supersecretpassword123'": "[REDACTED]",
		"AKIAIOSFODNN7EXAMPLE":              "[REDACTED]",
		"sk-1234567890abcdef1234567890":     "[REDACTED]",
	}
	for in, want := range cases {
		out := RedactSecrets(in)
		if !strings.Contains(out, want) {
			t.Errorf("redact %q -> %q, want contains %q", in, out, want)
		}
	}
}

func TestSummarizeDiffSmall(t *testing.T) {
	in := "diff --git a/foo b/foo\n+new"
	if got := SummarizeDiff(in); got != in {
		t.Errorf("small diff modified: %q", got)
	}
}
