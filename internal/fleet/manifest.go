package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestVersion is the current fleet manifest schema version.
const ManifestVersion = "1"

// Manifest represents the ~/.falcon/fleet.json file.
type Manifest struct {
	Version string      `json:"version"`
	Repos   []RepoEntry `json:"repos"`
}

// RepoEntry describes one repository in the fleet.
type RepoEntry struct {
	Path      string `json:"path"`
	Name      string `json:"name,omitempty"`
	Artifacts string `json:"artifacts,omitempty"`
}

// EffectiveName returns Name if set, otherwise the directory basename.
func (r RepoEntry) EffectiveName() string {
	if r.Name != "" {
		return r.Name
	}
	return filepath.Base(filepath.Clean(r.Path))
}

// EffectiveArtifactsDir returns the absolute path to the repo's artifacts.
func (r RepoEntry) EffectiveArtifactsDir() string {
	arts := r.Artifacts
	if arts == "" {
		arts = ".falcon/artifacts"
	}
	if filepath.IsAbs(arts) {
		return filepath.Clean(arts)
	}
	return filepath.Join(filepath.Clean(r.Path), arts)
}

// DefaultManifestPath returns ~/.falcon/fleet.json.
func DefaultManifestPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".falcon", "fleet.json"), nil
}

// LoadManifest reads and parses a fleet manifest from disk.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fleet manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse fleet manifest: %w", err)
	}
	if m.Version == "" {
		m.Version = ManifestVersion
	}
	return &m, nil
}

// ManifestFromPaths builds a Manifest from CLI-provided repo paths.
func ManifestFromPaths(paths []string) *Manifest {
	m := &Manifest{Version: ManifestVersion}
	for _, p := range paths {
		m.Repos = append(m.Repos, RepoEntry{Path: filepath.Clean(strings.TrimSpace(p))})
	}
	return m
}

// Validate checks that all repo paths exist.
func (m *Manifest) Validate() error {
	if len(m.Repos) == 0 {
		return fmt.Errorf("fleet manifest has no repos")
	}
	for i, r := range m.Repos {
		p := filepath.Clean(r.Path)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("repo %d (%s): path does not exist: %w", i, r.EffectiveName(), err)
		}
	}
	return nil
}
