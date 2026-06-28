// Package memory is falcon's deterministic "work memory": a feedback loop where
// an agent records the outcome of each graph query (useful / dead_end /
// corrected) and `reflect` distills those signals into a LESSONS file the next
// session preloads. No LLM, byte-stable output for a given input and `now`.
//
// Inspired by graphify's save-result/reflect doctrine: signals are scored (not
// counted) with time decay, and a node is only "preferred" once corroborated by
// several distinct useful results — one save can't mint a trusted lesson.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Outcomes are the work-memory signals a saved result may carry.
var Outcomes = []string{"useful", "dead_end", "corrected"}

// ValidOutcome reports whether s is a recognized outcome.
func ValidOutcome(s string) bool {
	for _, o := range Outcomes {
		if s == o {
			return true
		}
	}
	return false
}

// Record is one saved Q&A result.
type Record struct {
	Question   string    `json:"question"`
	Answer     string    `json:"answer,omitempty"`
	Type       string    `json:"type"`            // query | path | explain
	Nodes      []string  `json:"nodes,omitempty"` // cited symbol labels
	Outcome    string    `json:"outcome"`         // one of Outcomes
	Correction string    `json:"correction,omitempty"`
	Time       time.Time `json:"time"`
}

// Save writes a record as its own JSON file under dir (created if needed). The
// filename is content-addressed so identical re-saves are idempotent.
func Save(dir string, r Record) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(r.Time.UTC().Format(time.RFC3339Nano) + "\x00" + r.Question + "\x00" + r.Outcome))
	name := hex.EncodeToString(sum[:])[:16] + ".json"
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Load reads all records from dir, sorted by time then question for stable
// downstream ordering. A missing dir yields an empty slice.
func Load(dir string) ([]Record, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var recs []Record
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r Record
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		recs = append(recs, r)
	}
	sort.SliceStable(recs, func(i, j int) bool {
		if !recs[i].Time.Equal(recs[j].Time) {
			return recs[i].Time.Before(recs[j].Time)
		}
		return recs[i].Question < recs[j].Question
	})
	return recs, nil
}
