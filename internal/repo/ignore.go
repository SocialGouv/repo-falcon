package repo

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ignoreMatcher applies a set of .gitignore / .falconignore rules against
// repo-relative, slash-separated paths. It implements the common subset of the
// gitignore spec: comments (#), blank lines, negation (!), anchoring (leading
// /), directory-only rules (trailing /), and the *, **, ? wildcards. Matching is
// last-rule-wins, mirroring gitignore precedence.
//
// We parse the ignore files ourselves rather than shelling out to `git
// check-ignore` so behaviour is deterministic and works in environments without
// a .git directory (CI checkouts, Docker build contexts, exported tarballs) —
// the same choice graphify makes.
type ignoreMatcher struct {
	rules []ignoreRule
}

type ignoreRule struct {
	negate  bool
	dirOnly bool
	re      *regexp.Regexp
}

func (m *ignoreMatcher) empty() bool { return m == nil || len(m.rules) == 0 }

// Match reports whether relPath (slash-separated, repo-relative) is ignored.
// isDir must be true when relPath denotes a directory so directory-only rules
// (trailing /) apply correctly.
//
// A path is ignored if it, or any of its ancestor directories, matches a rule —
// mirroring git, where ignoring a directory ignores everything beneath it.
// Rules are evaluated in file order with last-match-wins so a later negation
// (!pattern) can re-include a path.
func (m *ignoreMatcher) Match(relPath string, isDir bool) bool {
	if m.empty() {
		return false
	}
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	if relPath == "" || relPath == "." {
		return false
	}

	segs := strings.Split(relPath, "/")
	ignored := false
	for i := 1; i <= len(segs); i++ {
		prefix := strings.Join(segs[:i], "/")
		// Every ancestor position is a directory; only the final component's
		// kind depends on isDir.
		isPosDir := i < len(segs) || isDir
		for _, r := range m.rules {
			if r.dirOnly && !isPosDir {
				continue
			}
			if r.re.MatchString(prefix) {
				ignored = !r.negate
			}
		}
	}
	return ignored
}

func newIgnoreMatcher(lines []string) *ignoreMatcher {
	m := &ignoreMatcher{}
	for _, raw := range lines {
		r, ok := compileIgnoreRule(raw)
		if ok {
			m.rules = append(m.rules, r)
		}
	}
	return m
}

// loadIgnoreFiles reads and merges the repo-root .gitignore and .falconignore.
// .gitignore is read first so a .falconignore entry (incl. a negation) can
// override it, matching gitignore's last-match-wins precedence.
func loadIgnoreFiles(repoRoot string) *ignoreMatcher {
	var lines []string
	for _, name := range []string{".gitignore", ".falconignore"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(bytes.NewReader(b))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
	}
	return newIgnoreMatcher(lines)
}

func compileIgnoreRule(raw string) (ignoreRule, bool) {
	line := strings.TrimRight(raw, " \t")
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	var negate bool
	if strings.HasPrefix(line, "!") {
		negate = true
		line = line[1:]
	}
	// An escaped leading '#' or '!' (\# \!) is a literal.
	if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
		line = line[1:]
	}
	var dirOnly bool
	if strings.HasSuffix(line, "/") {
		dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if line == "" {
		return ignoreRule{}, false
	}

	// A pattern anchored to the repo root either starts with '/' or contains a
	// non-trailing '/'. A pattern with no internal slash matches by basename at
	// any depth (gitignore semantics).
	anchored := strings.HasPrefix(line, "/") || strings.Contains(strings.TrimSuffix(line, "/"), "/")
	line = strings.TrimPrefix(line, "/")

	re, err := regexp.Compile(gitignoreToRegexp(line, anchored))
	if err != nil {
		return ignoreRule{}, false
	}
	return ignoreRule{negate: negate, dirOnly: dirOnly, re: re}, true
}

// gitignoreToRegexp converts a gitignore glob to an anchored regexp matching a
// full repo-relative path. Supported wildcards: ** (any path span), * (any run
// except /), ? (single char except /).
func gitignoreToRegexp(pat string, anchored bool) string {
	var b strings.Builder
	b.WriteString("^")
	if anchored {
		b.WriteString("(?:")
	} else {
		// Unanchored: allow any leading directories so the pattern matches by
		// basename at any depth.
		b.WriteString("(?:.*/)?(?:")
	}

	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch c {
		case '*':
			if i+1 < len(pat) && pat[i+1] == '*' {
				// '**' — spans across path separators.
				i++
				// '**/' collapses to "any number of leading dirs".
				if i+1 < len(pat) && pat[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}

	// Match a single path position exactly; subtree containment is handled by
	// the ancestor walk in Match.
	b.WriteString(")$")
	return b.String()
}
