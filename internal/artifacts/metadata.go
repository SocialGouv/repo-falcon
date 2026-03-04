package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Metadata struct {
	SchemaVersion string      `json:"schema_version"`
	Kind          string      `json:"kind"`
	Tool          Tool        `json:"tool"`
	Repo          Repo        `json:"repo"`
	Artifacts     Artifacts   `json:"artifacts"`
	Determinism   Determinism `json:"determinism"`
	Counts        *Counts     `json:"counts,omitempty"`
}

type Counts struct {
	Nodes    int `json:"nodes"`
	Files    int `json:"files"`
	Packages int `json:"packages"`
	Symbols  int `json:"symbols"`
	Findings int `json:"findings"`
	Edges    int `json:"edges"`
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Dirty   bool   `json:"dirty"`
}

type Repo struct {
	Root          string `json:"root"`
	VCS           string `json:"vcs"`
	Head          string `json:"head"`
	WorktreeClean bool   `json:"worktree_clean"`
}

type Artifacts struct {
	Path   string   `json:"path"`
	Tables []string `json:"tables"`
}

type Determinism struct {
	IDHash     string `json:"id_hash"`
	IDEncoding string `json:"id_encoding"`
	PathRoot   string `json:"path_root"`
	Timestamps string `json:"timestamps"`
}

func NewMinimalMetadata(repoRoot, artifactsPath string) Metadata {
	if repoRoot == "" {
		repoRoot = "."
	}
	if artifactsPath == "" {
		artifactsPath = "artifacts"
	}
	return Metadata{
		SchemaVersion: "1",
		Kind:          "index",
		Tool:          Tool{Name: "repofalcon", Version: "0.0.0", Commit: "", Dirty: false},
		Repo:          Repo{Root: repoRoot, VCS: "git", Head: "", WorktreeClean: false},
		Artifacts: Artifacts{Path: filepath.Clean(artifactsPath), Tables: []string{
			"nodes.parquet",
			"files.parquet",
			"packages.parquet",
			"symbols.parquet",
			"edges.parquet",
			"findings.parquet",
		}},
		Determinism: Determinism{IDHash: "sha256", IDEncoding: "hex", PathRoot: "repo-relative", Timestamps: "omitted"},
	}
}

func WriteMetadataJSON(path string, meta Metadata) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(meta); err != nil {
		return err
	}
	return nil
}

func ReadMetadataJSON(path string) (Metadata, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, err
	}
	var m Metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return Metadata{}, err
	}
	return m, nil
}
