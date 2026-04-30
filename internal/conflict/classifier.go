package conflict

import (
	"regexp"
	"strings"
)

type Complexity string

const (
	Trivial  Complexity = "trivial"
	Moderate Complexity = "moderate"
	Complex  Complexity = "complex"
)

func Classify(b Block) Complexity {
	if isWhitespaceOnly(b) {
		return Trivial
	}
	if isImportSortingOnly(b) {
		return Trivial
	}
	if isDuplicateAddition(b) {
		return Trivial
	}
	if isLockfileChurn(b) {
		return Trivial
	}

	if containsAuthOrSecurity(b) {
		return Complex
	}
	if hasDivergentControlFlow(b) {
		return Complex
	}
	if isSchemaOrMigration(b) {
		return Complex
	}
	if touchesTestsAndImpl(b) {
		return Complex
	}

	return Moderate
}

func normalize(lines []string) string {
	return strings.Join(lines, "\n")
}

func isWhitespaceOnly(b Block) bool {
	o := strings.Join(strings.Fields(strings.Join(b.OursLines, " ")), " ")
	t := strings.Join(strings.Fields(strings.Join(b.TheirsLines, " ")), " ")
	return o == t
}

var importRe = regexp.MustCompile(`^\s*(import|from|package|use|require)\b`)

func isImportSortingOnly(b Block) bool {
	check := func(ls []string) bool {
		for _, l := range ls {
			s := strings.TrimSpace(l)
			if s == "" {
				continue
			}
			if !importRe.MatchString(l) {
				return false
			}
		}
		return true
	}
	return check(b.OursLines) && check(b.TheirsLines)
}

func isDuplicateAddition(b Block) bool {
	o := normalize(b.OursLines)
	t := normalize(b.TheirsLines)
	if o == "" || t == "" {
		return false
	}
	return strings.Contains(o, t) || strings.Contains(t, o)
}

func isLockfileChurn(b Block) bool {
	p := strings.ToLower(b.FilePath)
	return strings.HasSuffix(p, "go.sum") ||
		strings.HasSuffix(p, "package-lock.json") ||
		strings.HasSuffix(p, "yarn.lock") ||
		strings.HasSuffix(p, "pnpm-lock.yaml") ||
		strings.HasSuffix(p, "cargo.lock") ||
		strings.HasSuffix(p, "poetry.lock")
}

var authPatterns = regexp.MustCompile(`(?i)\b(auth|token|secret|password|jwt|oauth|session|credential|crypto|hmac|signature|encrypt|decrypt)\b`)

func containsAuthOrSecurity(b Block) bool {
	if strings.Contains(b.FilePath, "auth/") || strings.Contains(b.FilePath, "security/") {
		return true
	}
	combined := normalize(b.OursLines) + "\n" + normalize(b.TheirsLines)
	return authPatterns.MatchString(combined)
}

var controlFlowRe = regexp.MustCompile(`\b(if|else|for|while|switch|return|throw|panic|defer|go func|async|await)\b`)

func hasDivergentControlFlow(b Block) bool {
	o := controlFlowRe.FindAllString(normalize(b.OursLines), -1)
	t := controlFlowRe.FindAllString(normalize(b.TheirsLines), -1)
	if len(o) == 0 && len(t) == 0 {
		return false
	}
	return abs(len(o)-len(t)) >= 2
}

func isSchemaOrMigration(b Block) bool {
	p := strings.ToLower(b.FilePath)
	if strings.Contains(p, "migration") || strings.Contains(p, "schema") {
		return true
	}
	combined := strings.ToLower(normalize(b.OursLines) + " " + normalize(b.TheirsLines))
	for _, kw := range []string{"create table", "alter table", "drop table", "create index", "add column", "drop column"} {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func touchesTestsAndImpl(b Block) bool {
	p := strings.ToLower(b.FilePath)
	isTest := strings.Contains(p, "_test.") || strings.Contains(p, ".test.") || strings.Contains(p, "/test/") || strings.Contains(p, "/tests/")
	if !isTest {
		return false
	}
	combined := normalize(b.OursLines) + "\n" + normalize(b.TheirsLines)
	return strings.Contains(combined, "func ") || strings.Contains(combined, "describe(") || strings.Contains(combined, "it(")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
