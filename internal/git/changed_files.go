package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"repofalcon/internal/graph"
)

// Change represents a single path-level change detected by git.
//
// Status is the first field of `git diff --name-status` output:
//   - "A" (added), "M" (modified), "D" (deleted)
//   - "Rxxx" (rename with score), "Cxxx" (copy with score)
//
// For renames/copies, OldPath is set and Path is the new path.
// Paths are repository-relative and slash-normalized.
type Change struct {
	Path    string
	OldPath string
	Status  string
}

// ChangedFiles returns deterministic path changes between base and head.
//
// It shells out to system git and parses NUL-delimited output from:
//
//	git diff --name-status -z <base> <head>
func ChangedFiles(repoRoot, base, head string) ([]Change, error) {
	base = strings.TrimSpace(base)
	head = strings.TrimSpace(head)
	if base == "" || head == "" {
		return nil, fmt.Errorf("git changed-files: base and head must be non-empty")
	}

	// Enable rename detection (otherwise renames may appear as delete+add depending on heuristics).
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--name-status", "-M", "-z", base, head)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff --name-status: %w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	// Tokens are NUL-separated, with a trailing NUL.
	toks := bytes.Split(out, []byte{0})
	if len(toks) > 0 && len(toks[len(toks)-1]) == 0 {
		toks = toks[:len(toks)-1]
	}

	var changes []Change
	for i := 0; i < len(toks); {
		status := string(toks[i])
		i++
		if status == "" {
			continue
		}

		isRenameOrCopy := strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")
		if isRenameOrCopy {
			if i+1 >= len(toks) {
				return nil, fmt.Errorf("parse git diff --name-status: truncated rename/copy record")
			}
			oldPath := string(toks[i])
			newPath := string(toks[i+1])
			i += 2

			oldCanon, err := graph.CanonRepoRelPath(oldPath)
			if err != nil {
				return nil, fmt.Errorf("canonicalize git old path %q: %w", oldPath, err)
			}
			newCanon, err := graph.CanonRepoRelPath(newPath)
			if err != nil {
				return nil, fmt.Errorf("canonicalize git new path %q: %w", newPath, err)
			}

			changes = append(changes, Change{Path: newCanon, OldPath: oldCanon, Status: status})
			continue
		}

		if i >= len(toks) {
			return nil, fmt.Errorf("parse git diff --name-status: truncated record")
		}
		path := string(toks[i])
		i++
		canon, err := graph.CanonRepoRelPath(path)
		if err != nil {
			return nil, fmt.Errorf("canonicalize git path %q: %w", path, err)
		}
		changes = append(changes, Change{Path: canon, Status: status})
	}

	// Deterministic ordering.
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		if changes[i].OldPath != changes[j].OldPath {
			return changes[i].OldPath < changes[j].OldPath
		}
		return changes[i].Status < changes[j].Status
	})

	return dedupeChanges(changes), nil
}

func dedupeChanges(in []Change) []Change {
	if len(in) <= 1 {
		return in
	}
	out := make([]Change, 0, len(in))
	prev := in[0]
	out = append(out, prev)
	for i := 1; i < len(in); i++ {
		c := in[i]
		if c.Path == prev.Path && c.OldPath == prev.OldPath && c.Status == prev.Status {
			continue
		}
		out = append(out, c)
		prev = c
	}
	return out
}
