package prpack

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ChangedFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldPath string `json:"old_path,omitempty"`
}

// ContextPack is written to pr_context_pack.json.
//
// Field order is intentionally fixed to keep JSON output deterministic.
type ContextPack struct {
	SchemaVersion    string            `json:"schema_version"`
	Kind             string            `json:"kind"`
	RepoRoot         string            `json:"repo_root"`
	SnapshotDir      string            `json:"snapshot_dir"`
	Base             string            `json:"base"`
	Head             string            `json:"head"`
	Artifacts        []string          `json:"artifacts"`
	ChangedFiles     []ChangedFile     `json:"changed_files"`
	ImpactedFiles    []ImpactedFile    `json:"impacted_files"`
	ImpactedSymbols  []ImpactedSymbol  `json:"impacted_symbols"`
	ImpactedPackages []ImpactedPackage `json:"impacted_packages"`
	Findings         []ImpactedFinding `json:"findings"`
}

func BuildContextPack(repoRoot, snapshotDir, base, head string, artifacts []string, changes []ChangedFile, impact ImpactResult) ContextPack {
	art := append([]string(nil), artifacts...)
	sort.Strings(art)
	art = dedupeStrings(art)

	chg := append([]ChangedFile(nil), changes...)
	sort.SliceStable(chg, func(i, j int) bool {
		if chg[i].Path != chg[j].Path {
			return chg[i].Path < chg[j].Path
		}
		if chg[i].OldPath != chg[j].OldPath {
			return chg[i].OldPath < chg[j].OldPath
		}
		return chg[i].Status < chg[j].Status
	})

	return ContextPack{
		SchemaVersion:    "1",
		Kind:             "pr_context_pack",
		RepoRoot:         filepath.Clean(strings.TrimSpace(repoRoot)),
		SnapshotDir:      filepath.Clean(strings.TrimSpace(snapshotDir)),
		Base:             strings.TrimSpace(base),
		Head:             strings.TrimSpace(head),
		Artifacts:        art,
		ChangedFiles:     chg,
		ImpactedFiles:    append([]ImpactedFile(nil), impact.ImpactedFiles...),
		ImpactedSymbols:  append([]ImpactedSymbol(nil), impact.ImpactedSymbols...),
		ImpactedPackages: append([]ImpactedPackage(nil), impact.ImpactedPackages...),
		Findings:         append([]ImpactedFinding(nil), impact.AttachedFindings...),
	}
}

func MarshalContextPack(pack ContextPack) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(pack); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func WriteContextPackJSON(path string, pack ContextPack) error {
	b, err := MarshalContextPack(pack)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
