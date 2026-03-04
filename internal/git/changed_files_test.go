package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestChangedFiles_BasicAndRename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git path behavior differs on windows")
	}

	repoRoot := t.TempDir()
	run(t, repoRoot, "git", "init")
	run(t, repoRoot, "git", "config", "user.email", "test@example.com")
	run(t, repoRoot, "git", "config", "user.name", "Test")

	writeFile(t, repoRoot, "a.txt", "one\n")
	run(t, repoRoot, "git", "add", ".")
	run(t, repoRoot, "git", "commit", "-m", "c1")

	writeFile(t, repoRoot, "a.txt", "two\n")
	writeFile(t, repoRoot, "b.txt", "b\n")
	run(t, repoRoot, "git", "add", ".")
	run(t, repoRoot, "git", "commit", "-m", "c2")

	run(t, repoRoot, "git", "mv", "b.txt", "c.txt")
	run(t, repoRoot, "git", "commit", "-am", "c3")

	// Diff from c2 -> c3 should surface a rename.
	changes, err := ChangedFiles(repoRoot, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("ChangedFiles (rename): %v", err)
	}
	var hasRename bool
	for _, c := range changes {
		if c.Path == "c.txt" && c.OldPath == "b.txt" && len(c.Status) > 0 && c.Status[0] == 'R' {
			hasRename = true
		}
	}
	if !hasRename {
		t.Fatalf("expected rename b.txt -> c.txt in changes: %#v", changes)
	}

	// Diff from c1 -> c3 should include the modified a.txt.
	changes2, err := ChangedFiles(repoRoot, "HEAD~2", "HEAD")
	if err != nil {
		t.Fatalf("ChangedFiles (range): %v", err)
	}
	var hasA bool
	for _, c := range changes2 {
		if c.Path == "a.txt" && c.Status == "M" {
			hasA = true
		}
	}
	if !hasA {
		t.Fatalf("expected modified a.txt in changes: %#v", changes2)
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v failed: %v\n%s", name, args, err, string(out))
	}
}

func writeFile(t *testing.T, repoRoot, rel, contents string) {
	t.Helper()
	path := filepath.Join(repoRoot, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
