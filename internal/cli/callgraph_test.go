package cli

import (
	"testing"

	"repofalcon/internal/extract"
)

func TestCallGraphResolver(t *testing.T) {
	b := newCallGraphBuilder()
	// Two packages, each with a "Helper" func, plus a unique "OnlyOne".
	b.addSymbol("Helper", "sym:pkgA.Helper", "pkgA", "go", "func")
	b.addSymbol("Helper", "sym:pkgB.Helper", "pkgB", "go", "func")
	b.addSymbol("OnlyOne", "sym:pkgB.OnlyOne", "pkgB", "go", "func")
	b.addSymbol("Caller", "sym:pkgA.Caller", "pkgA", "go", "func")

	b.addRefs("go", "pkgA", "file:a", map[string]string{"Caller": "sym:pkgA.Caller"}, []extract.Reference{
		{FromQualified: "Caller", Callee: "Helper", Kind: "call"},   // same-pkg unique -> pkgA.Helper @1.0
		{FromQualified: "Caller", Callee: "OnlyOne", Kind: "call"},  // global unique -> pkgB.OnlyOne @0.7
		{FromQualified: "Caller", Callee: "Nonexist", Kind: "call"}, // unresolved -> no edge
	})

	edges := b.resolveEdges()
	got := map[string]float32{}
	for _, e := range edges {
		if e.SrcID != "sym:pkgA.Caller" || e.EdgeType != "CALLS" {
			t.Errorf("unexpected edge %+v", e)
		}
		if e.Confidence != nil {
			got[e.DstID] = *e.Confidence
		}
	}

	if c, ok := got["sym:pkgA.Helper"]; !ok || c != 1.0 {
		t.Errorf("same-pkg Helper should resolve to pkgA.Helper @1.0, got %v ok=%v", c, ok)
	}
	if _, ok := got["sym:pkgB.Helper"]; ok {
		t.Error("must not resolve cross-pkg when a same-pkg match exists")
	}
	if c, ok := got["sym:pkgB.OnlyOne"]; !ok || c != 0.7 {
		t.Errorf("OnlyOne should resolve globally @0.7, got %v ok=%v", c, ok)
	}
	if len(edges) != 2 {
		t.Errorf("expected exactly 2 resolved edges, got %d", len(edges))
	}
}

func TestCallGraphAmbiguousDropped(t *testing.T) {
	b := newCallGraphBuilder()
	// "Amb" defined in two non-caller packages -> ambiguous global -> dropped.
	b.addSymbol("Amb", "sym:x.Amb", "x", "go", "func")
	b.addSymbol("Amb", "sym:y.Amb", "y", "go", "func")
	b.addSymbol("Caller", "sym:z.Caller", "z", "go", "func")
	b.addRefs("go", "z", "file:z", map[string]string{"Caller": "sym:z.Caller"}, []extract.Reference{
		{FromQualified: "Caller", Callee: "Amb", Kind: "call"},
	})
	if edges := b.resolveEdges(); len(edges) != 0 {
		t.Errorf("ambiguous call must be dropped, got %d edges", len(edges))
	}
}
