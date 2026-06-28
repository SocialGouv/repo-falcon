package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CommunityLabelsFile is the optional LLM-enrichment artifact, kept SEPARATE
// from the deterministic Parquet so the core graph stays reproducible. It maps a
// community representative symbol id to a human-readable name.
const CommunityLabelsFile = "community_labels.json"

// LoadCommunityLabels reads the labels overlay if present; a missing file is not
// an error (labels are optional).
func LoadCommunityLabels(snapshotDir string) map[string]string {
	b, err := os.ReadFile(filepath.Join(snapshotDir, CommunityLabelsFile))
	if err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

// SaveCommunityLabels writes the labels overlay.
func SaveCommunityLabels(snapshotDir string, labels map[string]string) error {
	b, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, CommunityLabelsFile), b, 0o644)
}
