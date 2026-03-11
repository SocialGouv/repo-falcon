package repo

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScan_IgnoreDirs(t *testing.T) {
	tmp := t.TempDir()

	// dirs to ignore
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".falcon"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, ".git", "x"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "node_modules", "y.js"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "artifacts", "z"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".falcon", "CONTEXT.md"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	// real files
	if err := os.WriteFile(filepath.Join(tmp, "a.go"), []byte("package p\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "b.py"), []byte("import os\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	recs, err := Scan(tmp, DefaultScanOptions())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	if recs[0].RepoRelPath != "a.go" {
		t.Fatalf("expected first path a.go, got %q", recs[0].RepoRelPath)
	}
	if recs[1].RepoRelPath != "sub/b.py" {
		t.Fatalf("expected second path sub/b.py, got %q", recs[1].RepoRelPath)
	}
}

func TestScan_SkipsPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permissions")
	}

	tmp := t.TempDir()

	// Readable file.
	if err := os.WriteFile(filepath.Join(tmp, "ok.go"), []byte("package ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unreadable file (permission denied on read).
	denied := filepath.Join(tmp, "secret.go")
	if err := os.WriteFile(denied, []byte("package secret\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(denied, 0o644) })

	// Unreadable directory (permission denied on listing).
	deniedDir := filepath.Join(tmp, "locked")
	if err := os.MkdirAll(deniedDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(deniedDir, 0o755) })

	recs, err := Scan(tmp, DefaultScanOptions())
	if err != nil {
		t.Fatalf("Scan should not fail on permission denied, got: %v", err)
	}

	if len(recs) != 1 {
		t.Fatalf("expected 1 record (ok.go only), got %d", len(recs))
	}
	if recs[0].RepoRelPath != "ok.go" {
		t.Fatalf("expected ok.go, got %q", recs[0].RepoRelPath)
	}
}
