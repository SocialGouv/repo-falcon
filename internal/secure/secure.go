// Package secure centralizes validation/sanitization of untrusted-ish input,
// mirroring graphify's security module doctrine (one place, not scattered).
// falcon ingests no URLs, so the live surfaces are: git refs passed to `git
// diff` (option-injection), output/label text, and path containment.
package secure

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidGitRef reports whether ref is safe to pass as a positional argument to
// git. It rejects empty refs, option injection (leading '-'), whitespace and
// control characters. It is intentionally conservative — a permissive superset
// of real branch/tag/SHA/`a..b` range syntax.
func ValidGitRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "-") {
		return false
	}
	for _, r := range ref {
		if r < 0x20 || r == 0x7f || r == ' ' || r == '\t' {
			return false
		}
	}
	return true
}

// SanitizeLabel makes a free-text label (LLM output, node name) safe to embed in
// reports: strips control characters, collapses whitespace to single spaces,
// drops surrounding quotes/backticks, and caps length.
func SanitizeLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, "\"'`.")
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = strings.TrimSpace(s[:80])
	}
	return s
}

// WithinBase resolves target and verifies it stays inside base, guarding against
// path traversal. Returns the cleaned absolute target.
func WithinBase(base, target string) (string, error) {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes base %q", target, base)
	}
	return targetAbs, nil
}
