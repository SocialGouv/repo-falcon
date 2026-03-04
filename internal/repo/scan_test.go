package repo

import (
	"os"
	"path/filepath"
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

	if err := os.WriteFile(filepath.Join(tmp, ".git", "x"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "node_modules", "y.js"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "artifacts", "z"), []byte("nope"), 0o644); err != nil {
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
