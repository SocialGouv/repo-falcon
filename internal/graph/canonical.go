package graph

import (
	"fmt"
	"path"
	"strings"
)

// CanonicalLanguage lowercases a language tag.
func CanonicalLanguage(lang string) string {
	return strings.ToLower(strings.TrimSpace(lang))
}

// CanonRepoRelPath canonicalizes a repository-relative path.
// Rules (per docs): clean, slash normalize, remove leading "./", forbid ".." segments.
func CanonRepoRelPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	// path.Clean always uses forward slashes.
	p = path.Clean(p)
	if p == "." {
		return "", fmt.Errorf("empty repo-relative path")
	}
	p = strings.TrimPrefix(p, "./")
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path must be repo-relative, got absolute: %q", p)
	}
	if p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") {
		return "", fmt.Errorf("path must not contain .. segments after cleaning: %q", p)
	}
	return p, nil
}

func MustCanonRepoRelPath(p string) string {
	pp, err := CanonRepoRelPath(p)
	if err != nil {
		panic(err)
	}
	return pp
}

// StableJoin joins fields with a newline delimiter for unambiguous hashing.
func StableJoin(fields ...string) string {
	return strings.Join(fields, "\n")
}
