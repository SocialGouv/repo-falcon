package graph

import "testing"

func TestFileIDDeterminism(t *testing.T) {
	t.Parallel()

	a := NewFileID("a/b/c.go")
	b := NewFileID("./a/b/../b/c.go")
	if a != b {
		t.Fatalf("expected IDs to match after canonicalization: %q != %q", a, b)
	}
}

func TestPackageIDDeterminism(t *testing.T) {
	t.Parallel()

	a := NewPackageID("GO", "example.com/mod")
	b := NewPackageID("go", "example.com/mod")
	if a != b {
		t.Fatalf("expected language canonicalization: %q != %q", a, b)
	}
}

func TestSymbolIDDeterminism(t *testing.T) {
	t.Parallel()

	id1 := NewSymbolID("Go", "internal/pkg", "pkg.F", "src/main.go", 10, 2, 12, 1)
	id2 := NewSymbolID("go", "internal/pkg", "pkg.F", "./src/./main.go", 10, 2, 12, 1)
	if id1 != id2 {
		t.Fatalf("expected symbol IDs to match after canonicalization: %q != %q", id1, id2)
	}
}

func TestFindingIDDeterminism(t *testing.T) {
	t.Parallel()

	id1 := NewFindingID("semgrep", "R1", "a/b.py", 5, 9, "hello   world")
	id2 := NewFindingID("semgrep", "R1", "./a/./b.py", 5, 9, "hello world")
	if id1 != id2 {
		t.Fatalf("expected finding IDs to match after canonicalization/message normalization: %q != %q", id1, id2)
	}
}

func TestEdgeIDDeterminism(t *testing.T) {
	t.Parallel()

	attrs1 := "site_file=src/x.go\nsite_sl=1\nsite_sc=2"
	attrs2 := "site_file=src/x.go\nsite_sl=1\nsite_sc=2\n" // whitespace should be trimmed
	ida := NewEdgeID("sha256:aaa", "sha256:bbb", EdgeCalls, attrs1)
	idb := NewEdgeID("sha256:aaa", "sha256:bbb", EdgeCalls, attrs2)
	if ida != idb {
		t.Fatalf("expected edge IDs to match: %q != %q", ida, idb)
	}
}
