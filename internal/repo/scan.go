package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"repofalcon/internal/graph"
)

// FileRecord is the result of scanning a single file in a repository.
// RepoRelPath is slash-normalized and deterministic.
type FileRecord struct {
	RepoRelPath   string
	AbsPath       string
	Extension     string
	Language      string
	SizeBytes     int64
	ContentSHA256 string
	Lines         int32
	Content       []byte // kept in-memory to feed extractors; not persisted.
}

type ScanOptions struct {
	// IgnoreDirNames are directory base names to skip entirely (e.g. ".git").
	IgnoreDirNames map[string]struct{}
}

func DefaultScanOptions() ScanOptions {
	return ScanOptions{IgnoreDirNames: defaultIgnoreDirs()}
}

func defaultIgnoreDirs() map[string]struct{} {
	// NOTE: base names only; applied at every level.
	ignored := []string{
		".git",
		"artifacts",
		"node_modules",
		"dist",
		"build",
		"target",
		"vendor",
		".venv",
		"__pycache__",
	}
	m := make(map[string]struct{}, len(ignored))
	for _, d := range ignored {
		m[d] = struct{}{}
	}
	return m
}

// Scan walks a repository root and returns deterministic file records.
// Ordering is lexicographic by RepoRelPath.
// Files and directories that cannot be read due to permission errors are
// silently skipped (a warning is logged).
func Scan(repoRoot string, opts ScanOptions) ([]FileRecord, error) {
	if repoRoot == "" {
		repoRoot = "."
	}
	repoRoot = filepath.Clean(repoRoot)

	lg := slog.Default()

	var out []FileRecord
	if err := walkSorted(repoRoot, "", opts, &out, lg); err != nil {
		return nil, err
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].RepoRelPath < out[j].RepoRelPath })
	return out, nil
}

func walkSorted(repoRoot, rel string, opts ScanOptions, out *[]FileRecord, lg *slog.Logger) error {
	ab := filepath.Join(repoRoot, rel)
	entries, err := os.ReadDir(ab)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			lg.Warn("skipping directory (permission denied)", "path", ab)
			return nil
		}
		return err
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		name := e.Name()
		// Skip symlinks (and other non-regular types) for safety and determinism.
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}

		relChild := filepath.Join(rel, name)
		if e.IsDir() {
			if _, ok := opts.IgnoreDirNames[name]; ok {
				continue
			}
			if err := walkSorted(repoRoot, relChild, opts, out, lg); err != nil {
				return err
			}
			continue
		}

		info, err := e.Info()
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				lg.Warn("skipping file (permission denied)", "path", filepath.Join(repoRoot, relChild))
				continue
			}
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}

		absPath := filepath.Join(repoRoot, relChild)
		b, err := os.ReadFile(absPath)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				lg.Warn("skipping file (permission denied)", "path", absPath)
				continue
			}
			return fmt.Errorf("read %s: %w", absPath, err)
		}

		repoRel := filepath.ToSlash(relChild)
		repoRel = path.Clean(repoRel)
		repoRel = strings.TrimPrefix(repoRel, "./")
		repoRel, err = graph.CanonRepoRelPath(repoRel)
		if err != nil {
			return fmt.Errorf("canonicalize path %q: %w", repoRel, err)
		}

		ext := strings.ToLower(filepath.Ext(name))
		lang := detectLanguageByExt(ext)
		sum := sha256.Sum256(b)
		shaHex := hex.EncodeToString(sum[:])
		lines := countLines(b)

		*out = append(*out, FileRecord{
			RepoRelPath:   repoRel,
			AbsPath:       absPath,
			Extension:     ext,
			Language:      lang,
			SizeBytes:     info.Size(),
			ContentSHA256: shaHex,
			Lines:         lines,
			Content:       b,
		})
	}

	return nil
}

func countLines(b []byte) int32 {
	if len(b) == 0 {
		return 0
	}
	var n int32 = 1
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}

func detectLanguageByExt(extLower string) string {
	// Keep language tags consistent with graph.CanonicalLanguage (lowercase).
	switch extLower {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "ts"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "js"
	case ".py":
		return "python"
	case ".java":
		return "java"
	default:
		return "unknown"
	}
}
