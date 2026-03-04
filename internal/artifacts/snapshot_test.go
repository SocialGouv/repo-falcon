package artifacts

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBuildSnapshot_MaterializesNodesAndValidatesEdgeEndpoints(t *testing.T) {
	ctx := context.Background()
	inDir := t.TempDir()
	outDir := t.TempDir()

	// Minimal index artifacts.
	fRows := []FileRow{{
		FileID:        "file-1",
		Path:          "a.go",
		Language:      "go",
		Extension:     ".go",
		SizeBytes:     1,
		ContentSHA256: "deadbeef",
		Lines:         nil,
		IsGenerated:   false,
		IsTest:        false,
	}}
	if err := WriteFilesParquet(filepath.Join(inDir, "files.parquet"), fRows); err != nil {
		t.Fatalf("write files: %v", err)
	}
	if err := WritePackagesParquet(filepath.Join(inDir, "packages.parquet"), nil); err != nil {
		t.Fatalf("write packages: %v", err)
	}
	if err := WriteSymbolsParquet(filepath.Join(inDir, "symbols.parquet"), nil); err != nil {
		t.Fatalf("write symbols: %v", err)
	}
	if err := WriteFindingsParquet(filepath.Join(inDir, "findings.parquet"), nil); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	// One edge references an existing node and one missing node.
	eRows := []EdgeRow{{
		EdgeID:   "edge-1",
		EdgeType: "IMPORTS",
		SrcID:    "file-1",
		DstID:    "pkg-missing",
		SrcType:  "File",
		DstType:  "Package",
	}}
	if err := WriteEdgesParquet(filepath.Join(inDir, "edges.parquet"), eRows); err != nil {
		t.Fatalf("write edges: %v", err)
	}

	counts, err := BuildSnapshot(ctx, inDir, outDir)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if counts.Edges != 1 {
		t.Fatalf("expected 1 edge, got %d", counts.Edges)
	}
	if counts.Nodes < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", counts.Nodes)
	}

	nodes, err := ReadNodesParquet(ctx, filepath.Join(outDir, "nodes.parquet"))
	if err != nil {
		t.Fatalf("read nodes: %v", err)
	}
	edges, err := ReadEdgesParquet(ctx, filepath.Join(outDir, "edges.parquet"))
	if err != nil {
		t.Fatalf("read edges: %v", err)
	}

	nodeIDs := map[string]bool{}
	for _, n := range nodes {
		if n.NodeID == "" {
			t.Fatalf("empty node_id")
		}
		nodeIDs[n.NodeID] = true
	}
	for _, e := range edges {
		if !nodeIDs[e.SrcID] {
			t.Fatalf("missing src node for edge %s: %s", e.EdgeID, e.SrcID)
		}
		if !nodeIDs[e.DstID] {
			t.Fatalf("missing dst node for edge %s: %s", e.EdgeID, e.DstID)
		}
	}
}

func TestStableAttrsJSON_KeyOrderingDeterministic(t *testing.T) {
	// Intentionally provide keys in a non-sorted order.
	j1 := stableAttrsJSON(map[string]any{
		"z":           3,
		"a":           "x",
		"m":           map[string]any{"b": 2, "a": 1},
		"nil_omitted": nil,
	})
	j2 := stableAttrsJSON(map[string]any{
		"m":           map[string]any{"a": 1, "b": 2},
		"a":           "x",
		"z":           3,
		"nil_omitted": nil,
	})
	if j1 != j2 {
		t.Fatalf("expected stable JSON across map orderings, got\n1=%s\n2=%s", j1, j2)
	}
	// Top-level keys must be in lexical order: a, m, z.
	expected := `{"a":"x","m":{"a":1,"b":2},"z":3}`
	if j1 != expected {
		t.Fatalf("unexpected JSON encoding\nexpected=%s\ngot=%s", expected, j1)
	}
}
