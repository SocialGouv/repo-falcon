package prpack

import (
	"testing"

	"repofalcon/internal/artifacts"
	"repofalcon/internal/graph"
)

func TestComputeImpact_SyntheticGraph(t *testing.T) {
	fa := artifacts.FileRow{FileID: graph.NewFileID("a.go"), Path: "a.go", Language: "go", Extension: ".go", SizeBytes: 1, ContentSHA256: "sha", Lines: nil, IsGenerated: false, IsTest: false}
	fb := artifacts.FileRow{FileID: graph.NewFileID("b.go"), Path: "b.go", Language: "go", Extension: ".go", SizeBytes: 1, ContentSHA256: "sha", Lines: nil, IsGenerated: false, IsTest: false}

	pkg1 := artifacts.PackageRow{PackageID: graph.NewPackageID("go", "example.com/p"), Ecosystem: "go", Scope: "", Name: "example.com/p", Version: "", IsInternal: true}
	pkg2 := artifacts.PackageRow{PackageID: graph.NewPackageID("go", "fmt"), Ecosystem: "go", Scope: "", Name: "fmt", Version: "", IsInternal: false}

	pkg1id := pkg1.PackageID
	s1 := artifacts.SymbolRow{SymbolID: graph.NewSymbolID("go", "example.com/p", "A", "a.go", 1, 1, 1, 2), FileID: fa.FileID, PackageID: &pkg1id, Language: "go", Kind: "type", Name: "A", QualifiedName: "A", SemanticKey: "k", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 2}

	edges := []artifacts.EdgeRow{
		{EdgeID: "e1", EdgeType: string(graph.EdgeContains), SrcID: pkg1.PackageID, SrcType: string(graph.NodeTypePackage), DstID: fa.FileID, DstType: string(graph.NodeTypeFile)},
		{EdgeID: "e2", EdgeType: string(graph.EdgeContains), SrcID: pkg1.PackageID, SrcType: string(graph.NodeTypePackage), DstID: fb.FileID, DstType: string(graph.NodeTypeFile)},
		{EdgeID: "e3", EdgeType: string(graph.EdgeImports), SrcID: fa.FileID, SrcType: string(graph.NodeTypeFile), DstID: pkg2.PackageID, DstType: string(graph.NodeTypePackage)},
		{EdgeID: "e4", EdgeType: string(graph.EdgeDefines), SrcID: fa.FileID, SrcType: string(graph.NodeTypeFile), DstID: s1.SymbolID, DstType: string(graph.NodeTypeSymbol)},
		{EdgeID: "e5", EdgeType: string(graph.EdgeInFile), SrcID: s1.SymbolID, SrcType: string(graph.NodeTypeSymbol), DstID: fa.FileID, DstType: string(graph.NodeTypeFile)},
	}

	fidB := fb.FileID
	findings := []artifacts.FindingRow{{FindingID: graph.NewFindingID("tool", "R1", "b.go", 1, 1, "msg"), SourceTool: "tool", RuleID: "R1", Severity: "HIGH", Message: "msg", MessageFingerprint: "fp", FileID: &fidB}}

	res, err := ComputeImpact(SnapshotTables{
		Files:    []artifacts.FileRow{fa, fb},
		Packages: []artifacts.PackageRow{pkg1, pkg2},
		Symbols:  []artifacts.SymbolRow{s1},
		Edges:    edges,
		Findings: findings,
	}, []string{"a.go"}, ImpactOptions{MaxDepth: 2})
	if err != nil {
		t.Fatalf("ComputeImpact: %v", err)
	}

	gotFiles := map[string]bool{}
	for _, f := range res.ImpactedFiles {
		gotFiles[f.Path] = true
	}
	if !gotFiles["a.go"] || !gotFiles["b.go"] {
		t.Fatalf("expected impacted files a.go and b.go, got: %#v", res.ImpactedFiles)
	}

	gotPkgs := map[string]bool{}
	for _, p := range res.ImpactedPackages {
		gotPkgs[p.PackageID] = true
	}
	if !gotPkgs[pkg1.PackageID] || !gotPkgs[pkg2.PackageID] {
		t.Fatalf("expected impacted packages pkg1 and pkg2, got: %#v", res.ImpactedPackages)
	}

	if len(res.ImpactedSymbols) != 1 || res.ImpactedSymbols[0].SymbolID != s1.SymbolID {
		t.Fatalf("expected impacted symbol %s, got: %#v", s1.SymbolID, res.ImpactedSymbols)
	}

	if len(res.AttachedFindings) != 1 {
		t.Fatalf("expected 1 finding, got: %#v", res.AttachedFindings)
	}
}

func TestComputeImpact_ChangedFileNotInSnapshot(t *testing.T) {
	res, err := ComputeImpact(SnapshotTables{}, []string{"new.go"}, ImpactOptions{MaxDepth: 2})
	if err != nil {
		t.Fatalf("ComputeImpact: %v", err)
	}
	if len(res.ImpactedFiles) != 1 {
		t.Fatalf("expected 1 impacted file, got: %#v", res.ImpactedFiles)
	}
	if res.ImpactedFiles[0].Path != "new.go" {
		t.Fatalf("expected impacted file path new.go, got: %#v", res.ImpactedFiles[0])
	}
	if res.ImpactedFiles[0].InSnapshot {
		t.Fatalf("expected InSnapshot=false, got: %#v", res.ImpactedFiles[0])
	}
}

func TestComputeImpact_BFSDepthAndEdgeFiltering(t *testing.T) {
	// Graph:
	//   a.go --IMPORTS--> pkgX --CONTAINS--> b.go
	//   a.go --REFERENCES--> c.go   (should be ignored)
	fa := artifacts.FileRow{FileID: graph.NewFileID("a.go"), Path: "a.go", Language: "go", Extension: ".go", SizeBytes: 1, ContentSHA256: "sha", Lines: nil, IsGenerated: false, IsTest: false}
	fb := artifacts.FileRow{FileID: graph.NewFileID("b.go"), Path: "b.go", Language: "go", Extension: ".go", SizeBytes: 1, ContentSHA256: "sha", Lines: nil, IsGenerated: false, IsTest: false}
	fc := artifacts.FileRow{FileID: graph.NewFileID("c.go"), Path: "c.go", Language: "go", Extension: ".go", SizeBytes: 1, ContentSHA256: "sha", Lines: nil, IsGenerated: false, IsTest: false}

	pkgX := artifacts.PackageRow{PackageID: graph.NewPackageID("go", "example.com/x"), Ecosystem: "go", Scope: "", Name: "example.com/x", Version: "", IsInternal: true}

	edges := []artifacts.EdgeRow{
		{EdgeID: "e1", EdgeType: string(graph.EdgeImports), SrcID: fa.FileID, SrcType: string(graph.NodeTypeFile), DstID: pkgX.PackageID, DstType: string(graph.NodeTypePackage)},
		{EdgeID: "e2", EdgeType: string(graph.EdgeContains), SrcID: pkgX.PackageID, SrcType: string(graph.NodeTypePackage), DstID: fb.FileID, DstType: string(graph.NodeTypeFile)},
		{EdgeID: "e3", EdgeType: string(graph.EdgeReferences), SrcID: fa.FileID, SrcType: string(graph.NodeTypeFile), DstID: fc.FileID, DstType: string(graph.NodeTypeFile)},
	}

	// Depth=1: a.go can reach pkgX, but not b.go (which is depth 2), and must not reach c.go via REFERENCES.
	res1, err := ComputeImpact(SnapshotTables{Files: []artifacts.FileRow{fa, fb, fc}, Packages: []artifacts.PackageRow{pkgX}, Edges: edges}, []string{"a.go"}, ImpactOptions{MaxDepth: 1})
	if err != nil {
		t.Fatalf("ComputeImpact depth=1: %v", err)
	}
	got1 := map[string]bool{}
	for _, f := range res1.ImpactedFiles {
		got1[f.Path] = true
	}
	if !got1["a.go"] {
		t.Fatalf("expected a.go impacted at depth=1")
	}
	if got1["b.go"] {
		t.Fatalf("did not expect b.go impacted at depth=1")
	}
	if got1["c.go"] {
		t.Fatalf("did not expect c.go impacted via REFERENCES")
	}

	// Depth=2: b.go becomes reachable via IMPORTS+CONTAINS.
	res2, err := ComputeImpact(SnapshotTables{Files: []artifacts.FileRow{fa, fb, fc}, Packages: []artifacts.PackageRow{pkgX}, Edges: edges}, []string{"a.go"}, ImpactOptions{MaxDepth: 2})
	if err != nil {
		t.Fatalf("ComputeImpact depth=2: %v", err)
	}
	got2 := map[string]bool{}
	for _, f := range res2.ImpactedFiles {
		got2[f.Path] = true
	}
	if !got2["b.go"] {
		t.Fatalf("expected b.go impacted at depth=2")
	}
	if got2["c.go"] {
		t.Fatalf("did not expect c.go impacted via REFERENCES even at depth=2")
	}
}
