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
	// Ignore applies repo-root .gitignore / .falconignore rules. May be nil.
	Ignore *ignoreMatcher
	// SkipGeneratedJS drops minified / bundled JS-TS files (huge single-line
	// builds that pollute the symbol graph and slow extraction with no value).
	SkipGeneratedJS bool
}

func DefaultScanOptions() ScanOptions {
	return ScanOptions{IgnoreDirNames: defaultIgnoreDirs(), SkipGeneratedJS: true}
}

func defaultIgnoreDirs() map[string]struct{} {
	// NOTE: base names only; applied at every level.
	ignored := []string{
		".git",
		".hg",
		".svn",
		".falcon",
		"artifacts",
		"node_modules",
		"dist",
		"build",
		"out",
		"target",
		"vendor",
		".venv",
		"venv",
		"__pycache__",
		// Tool/agent caches and runtime state that are never source.
		".claude",
		".iterion",
		".graphify",
		"graphify-out",
		".cache",
		".npm",
		".yarn",
		".pnpm-store",
		".gradle",
		".idea",
		".vscode",
		".next",
		".nuxt",
		".turbo",
		".terraform",
		"coverage",
		".nyc_output",
		".pytest_cache",
		".mypy_cache",
		".ruff_cache",
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

	// Respect repo-root .gitignore / .falconignore unless the caller supplied
	// its own matcher.
	if opts.Ignore == nil {
		opts.Ignore = loadIgnoreFiles(repoRoot)
	}

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
		relSlash := filepath.ToSlash(relChild)
		if e.IsDir() {
			if _, ok := opts.IgnoreDirNames[name]; ok {
				continue
			}
			if opts.Ignore.Match(relSlash, true) {
				continue
			}
			// Skip nested git repositories / worktrees / submodules: a subdir
			// that is itself a checkout (has a .git entry) belongs to another
			// repo and must be indexed on its own, not folded into this one.
			// This is what excludes sibling worktrees (e.g. .works/*,
			// .claude/worktrees/*) without hard-coding their names.
			if rel != "" {
				if _, err := os.Lstat(filepath.Join(ab, name, ".git")); err == nil {
					continue
				}
			}
			if err := walkSorted(repoRoot, relChild, opts, out, lg); err != nil {
				return err
			}
			continue
		}

		if opts.Ignore.Match(relSlash, false) {
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

		// Drop minified / bundled JS-TS: name markers, or the structural
		// signature of a build (very high bytes-per-line). These carry no useful
		// symbols and slow extraction. Source maps are skipped outright.
		if opts.SkipGeneratedJS && (lang == "js" || lang == "ts") && isGeneratedJS(name, info.Size(), lines) {
			lg.Warn("skipping generated/minified bundle", "path", absPath)
			continue
		}

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

// isGeneratedJS reports whether a JS/TS file is a minified bundle or build
// artifact rather than authored source. Two signals: well-known name markers,
// and the structural signature of minification (a small line count over a large
// byte size — bundlers emit a few enormous lines).
func isGeneratedJS(name string, size int64, lines int32) bool {
	lower := strings.ToLower(name)
	for _, suf := range []string{".min.js", ".min.ts", ".bundle.js", ".chunk.js", ".js.map", ".ts.map", ".min.mjs", ".min.cjs"} {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	// Minification signature: > 50 KB packed into very few lines. Gated to
	// JS/TS extensions so the heuristic can't misfire on other languages.
	isJSExt := false
	for _, e := range []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"} {
		if strings.HasSuffix(lower, e) {
			isJSExt = true
			break
		}
	}
	if isJSExt && size > 50*1024 && lines > 0 && size/int64(lines) > 2000 {
		return true
	}
	return false
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
