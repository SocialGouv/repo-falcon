package extract

import "testing"

func TestExtractGoFile_PackageImportsSymbols(t *testing.T) {
	src := []byte(`package main

import (
  "fmt"
  ali "os"
)

const C = 1
var V = 2

type T struct{}

func F() {}
func (t *T) M() {}
`)

	got, err := ExtractGoFile("x.go", src)
	if err != nil {
		t.Fatalf("ExtractGoFile error: %v", err)
	}
	if got.PackageName != "main" {
		t.Fatalf("package: expected main, got %q", got.PackageName)
	}
	if len(got.Imports) != 2 || got.Imports[0] != "fmt" || got.Imports[1] != "os" {
		t.Fatalf("imports: expected [fmt os], got %#v", got.Imports)
	}

	// Names only (positions are still validated for presence > 0).
	wantKinds := map[string]string{
		"C": "const",
		"V": "var",
		"T": "type",
		"F": "func",
		"M": "method",
	}
	for _, s := range got.Symbols {
		wk, ok := wantKinds[s.Name]
		if !ok {
			continue
		}
		if s.Kind != wk {
			t.Fatalf("symbol %s: expected kind %q, got %q", s.Name, wk, s.Kind)
		}
		if s.StartLine <= 0 || s.StartCol <= 0 {
			t.Fatalf("symbol %s: expected positive start pos, got %d:%d", s.Name, s.StartLine, s.StartCol)
		}
	}
}
