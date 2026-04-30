package ai

import (
	"fmt"
	"regexp"
	"strings"
)

const maxDiffChars = 4000

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token|bearer)\s*[:=]\s*['"]?[A-Za-z0-9_\-\.]{16,}['"]?`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----[\s\S]+?-----END[^\n]+-----`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]+`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
}

func RedactSecrets(s string) string {
	out := s
	for _, p := range secretPatterns {
		out = p.ReplaceAllString(out, "[REDACTED]")
	}
	return out
}

func SummarizeDiff(diff string) string {
	diff = RedactSecrets(diff)
	if len(diff) <= maxDiffChars {
		return diff
	}

	files := splitByFile(diff)
	var b strings.Builder
	fmt.Fprintf(&b, "Changed files (%d):\n", len(files))
	for _, f := range files {
		fmt.Fprintf(&b, "- %s (+%d -%d): %s\n", f.path, f.added, f.removed, f.preview)
	}
	if b.Len() > maxDiffChars {
		return b.String()[:maxDiffChars]
	}
	return b.String()
}

type fileSummary struct {
	path    string
	added   int
	removed int
	preview string
}

func splitByFile(diff string) []fileSummary {
	lines := strings.Split(diff, "\n")
	var out []fileSummary
	var cur *fileSummary
	pathRe := regexp.MustCompile(`^diff --git a/(\S+) b/(\S+)`)
	previewKept := 0
	for _, line := range lines {
		if m := pathRe.FindStringSubmatch(line); m != nil {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &fileSummary{path: m[2]}
			previewKept = 0
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			cur.added++
			if previewKept < 8 {
				if cur.preview != "" {
					cur.preview += " | "
				}
				cur.preview += truncate(strings.TrimSpace(line), 60)
				previewKept++
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			cur.removed++
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func CompressStepHistory(steps []map[string]any) string {
	if len(steps) <= 3 {
		buf, _ := jsonStringify(steps)
		return buf
	}
	keep := steps[len(steps)-3:]
	older := steps[:len(steps)-3]
	var b strings.Builder
	for _, s := range older {
		fmt.Fprintf(&b, "- step: action=%v score=%v\n", s["action"], s["score"])
	}
	b.WriteString("recent_steps: ")
	recent, _ := jsonStringify(keep)
	b.WriteString(recent)
	return b.String()
}

func jsonStringify(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
