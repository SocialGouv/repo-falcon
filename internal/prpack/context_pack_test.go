package prpack

import (
	"strings"
	"testing"
)

func TestMarshalContextPack_SortsArraysDeterministically(t *testing.T) {
	pack := ContextPack{
		SchemaVersion: "1",
		Kind:          "pr_context_pack",
		RepoRoot:      ".",
		SnapshotDir:   "artifacts",
		Base:          "base",
		Head:          "head",
		Artifacts:     []string{"symbols.parquet", "files.parquet"},
		ChangedFiles:  []ChangedFile{{Path: "b.go", Status: "M"}, {Path: "a.go", Status: "A"}},
		ImpactedFiles: []ImpactedFile{{Path: "b.go", FileID: "2", InSnapshot: true}, {Path: "a.go", FileID: "1", InSnapshot: true}},
	}

	pack = BuildContextPack(".", "artifacts", "base", "head", pack.Artifacts, pack.ChangedFiles, ImpactResult{ImpactedFiles: pack.ImpactedFiles})
	b, err := MarshalContextPack(pack)
	if err != nil {
		t.Fatalf("MarshalContextPack: %v", err)
	}
	s := string(b)

	ia := strings.Index(s, "\"path\": \"a.go\"")
	ib := strings.Index(s, "\"path\": \"b.go\"")
	if ia == -1 || ib == -1 || ia > ib {
		t.Fatalf("expected a.go before b.go in JSON, got:\n%s", s)
	}
}
