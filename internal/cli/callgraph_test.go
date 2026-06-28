package cli

import (
	"strings"
	"testing"

	"repofalcon/internal/extract"
)

func TestCallGraphResolver(t *testing.T) {
	b := newCallGraphBuilder()
	// Two packages, each with a "Helper" func, plus a unique "OnlyOne".
	b.addSymbol("Helper", "Helper", "sym:pkgA.Helper", "pkgA", "go", "func")
	b.addSymbol("Helper", "Helper", "sym:pkgB.Helper", "pkgB", "go", "func")
	b.addSymbol("OnlyOne", "OnlyOne", "sym:pkgB.OnlyOne", "pkgB", "go", "func")
	b.addSymbol("Caller", "Caller", "sym:pkgA.Caller", "pkgA", "go", "func")

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
	if c, ok := got["sym:pkgB.OnlyOne"]; !ok || c != 0.75 {
		t.Errorf("OnlyOne should resolve globally @0.75, got %v ok=%v", c, ok)
	}
	if len(edges) != 2 {
		t.Errorf("expected exactly 2 resolved edges, got %d", len(edges))
	}
}

func TestCallGraphAmbiguousFanout(t *testing.T) {
	// A small ambiguity (2 candidates, none in caller's pkg) is emitted to both,
	// flagged AMBIGUOUS @0.5, rather than silently dropped.
	b := newCallGraphBuilder()
	b.addSymbol("Amb", "Amb", "sym:x.Amb", "x", "go", "func")
	b.addSymbol("Amb", "Amb", "sym:y.Amb", "y", "go", "func")
	b.addSymbol("Caller", "Caller", "sym:z.Caller", "z", "go", "func")
	b.addRefs("go", "z", "file:z", map[string]string{"Caller": "sym:z.Caller"}, []extract.Reference{
		{FromQualified: "Caller", Callee: "Amb", Kind: "call"},
	})
	edges := b.resolveEdges()
	if len(edges) != 2 {
		t.Fatalf("small ambiguity should fan out to 2 flagged edges, got %d", len(edges))
	}
	for _, e := range edges {
		if e.Confidence == nil || *e.Confidence != 0.5 {
			t.Errorf("ambiguous edge should be @0.5, got %v", e.Confidence)
		}
		if e.PropertiesJSON == nil || !strings.Contains(*e.PropertiesJSON, "AMBIGUOUS") {
			t.Errorf("ambiguous edge should be flagged AMBIGUOUS, got %v", e.PropertiesJSON)
		}
	}
}

func TestCallGraphTooAmbiguousDropped(t *testing.T) {
	// Beyond the fan-out cap, a name is too ambiguous to be useful -> dropped.
	b := newCallGraphBuilder()
	for _, p := range []string{"a", "b", "c", "d", "e"} {
		b.addSymbol("Wide", "Wide", "sym:"+p+".Wide", p, "go", "func")
	}
	b.addSymbol("Caller", "Caller", "sym:z.Caller", "z", "go", "func")
	b.addRefs("go", "z", "file:z", map[string]string{"Caller": "sym:z.Caller"}, []extract.Reference{
		{FromQualified: "Caller", Callee: "Wide", Kind: "call"},
	})
	if edges := b.resolveEdges(); len(edges) != 0 {
		t.Errorf("over-cap ambiguity must be dropped, got %d edges", len(edges))
	}
}

func TestCallGraphReceiverTyping(t *testing.T) {
	b := newCallGraphBuilder()
	// Two distinct types each with a method "Run"; a bare-name resolver would
	// see "Run" as ambiguous and drop it. Receiver typing disambiguates.
	b.addSymbol("Run", "A.Run", "sym:p.A.Run", "p", "go", "method")
	b.addSymbol("Run", "B.Run", "sym:p.B.Run", "p", "go", "method")
	b.addSymbol("Caller", "Caller", "sym:p.Caller", "p", "go", "func")
	b.addRefs("go", "p", "file:p", map[string]string{"Caller": "sym:p.Caller"}, []extract.Reference{
		{FromQualified: "Caller", Callee: "Run", RecvType: "B", Kind: "call"},
	})
	edges := b.resolveEdges()
	if len(edges) != 1 || edges[0].DstID != "sym:p.B.Run" {
		t.Fatalf("receiver typing should resolve to B.Run, got %+v", edges)
	}
}

func TestCallGraphTypeReference(t *testing.T) {
	b := newCallGraphBuilder()
	// A "reference" must resolve to a type symbol, not a like-named function.
	b.addSymbol("Config", "Config", "sym:p.Config.fn", "p", "go", "func")
	b.addSymbol("Config", "Config", "sym:p.Config.type", "p", "go", "type")
	b.addSymbol("User", "User", "sym:p.User", "p", "go", "func")
	b.addRefs("go", "p", "file:p", map[string]string{"User": "sym:p.User"}, []extract.Reference{
		{FromQualified: "User", Callee: "Config", Kind: "reference"},
	})
	edges := b.resolveEdges()
	if len(edges) != 1 || edges[0].EdgeType != "REFERENCES" || edges[0].DstID != "sym:p.Config.type" {
		t.Fatalf("reference should resolve to the type symbol, got %+v", edges)
	}
}
