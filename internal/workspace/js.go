package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// detectJSWorkspaces detects npm/yarn/pnpm workspaces.
func detectJSWorkspaces(repoRoot string) []WorkspaceMember {
	patterns := readJSWorkspacePatterns(repoRoot)
	if len(patterns) == 0 {
		return nil
	}

	var members []WorkspaceMember
	for _, pattern := range patterns {
		dirs := expandGlob(repoRoot, pattern)
		for _, dir := range dirs {
			m := readJSMember(repoRoot, dir)
			if m != nil {
				members = append(members, *m)
			}
		}
	}
	return members
}

// readJSWorkspacePatterns reads workspace glob patterns from package.json or pnpm-workspace.yaml.
func readJSWorkspacePatterns(repoRoot string) []string {
	// Try package.json first (npm/yarn).
	if patterns := readPackageJSONWorkspaces(repoRoot); len(patterns) > 0 {
		return patterns
	}
	// Try pnpm-workspace.yaml.
	if patterns := readPnpmWorkspaceYAML(repoRoot); len(patterns) > 0 {
		return patterns
	}
	return nil
}

func readPackageJSONWorkspaces(repoRoot string) []string {
	data, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return nil
	}

	// Parse the workspaces field, which can be:
	// - an array of strings: ["packages/*", "apps/*"]
	// - an object with packages: { "packages": ["packages/*"] }
	var pkg struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Workspaces == nil {
		return nil
	}

	// Try array form.
	var arr []string
	if json.Unmarshal(pkg.Workspaces, &arr) == nil && len(arr) > 0 {
		return arr
	}

	// Try object form.
	var obj struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(pkg.Workspaces, &obj) == nil && len(obj.Packages) > 0 {
		return obj.Packages
	}

	return nil
}

// readPnpmWorkspaceYAML does minimal line-based parsing of pnpm-workspace.yaml.
// Format:
//
//	packages:
//	  - "packages/*"
//	  - "apps/*"
func readPnpmWorkspaceYAML(repoRoot string) []string {
	data, err := os.ReadFile(filepath.Join(repoRoot, "pnpm-workspace.yaml"))
	if err != nil {
		return nil
	}

	var patterns []string
	inPackages := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "packages:" {
			inPackages = true
			continue
		}
		if inPackages {
			if strings.HasPrefix(trimmed, "- ") {
				val := strings.TrimPrefix(trimmed, "- ")
				val = strings.Trim(val, `"'`)
				if val != "" {
					patterns = append(patterns, val)
				}
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				// Hit a new top-level key, stop.
				break
			}
		}
	}
	return patterns
}

// expandGlob expands a workspace glob pattern into matching directories.
// Supports simple patterns like "packages/*" and "apps/*".
// For "**" patterns, does a recursive directory walk.
func expandGlob(repoRoot, pattern string) []string {
	// Normalize: strip trailing / and handle ** → recursive walk.
	pattern = strings.TrimSuffix(pattern, "/")

	if strings.Contains(pattern, "**") {
		return expandDoubleStarGlob(repoRoot, pattern)
	}

	fullPattern := filepath.Join(repoRoot, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && info.IsDir() {
			rel, err := filepath.Rel(repoRoot, m)
			if err == nil {
				dirs = append(dirs, rel)
			}
		}
	}
	return dirs
}

// expandDoubleStarGlob handles patterns with "**" by walking subdirectories.
func expandDoubleStarGlob(repoRoot, pattern string) []string {
	// Replace "**" with single-level matching, walk directories.
	// For patterns like "packages/**", find all immediate subdirs of "packages/".
	base := strings.SplitN(pattern, "**", 2)[0]
	base = strings.TrimSuffix(base, "/")

	baseDir := filepath.Join(repoRoot, base)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "node_modules" {
			dirs = append(dirs, filepath.Join(base, e.Name()))
		}
	}
	return dirs
}

// readJSMember reads a workspace member's package.json to get its name.
func readJSMember(repoRoot, relDir string) *WorkspaceMember {
	pkgPath := filepath.Join(repoRoot, relDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}

	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Name == "" {
		return nil
	}

	manifestRel := filepath.Join(relDir, "package.json")
	return &WorkspaceMember{
		Name:         pkg.Name,
		RootPath:     relDir,
		ManifestPath: manifestRel,
		Ecosystem:    "npm",
		PackageNames: []string{pkg.Name},
	}
}
