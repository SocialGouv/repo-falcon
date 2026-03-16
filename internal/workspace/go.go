package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// detectGoWorkspaces detects Go workspaces via go.work files.
func detectGoWorkspaces(repoRoot string) []WorkspaceMember {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.work"))
	if err != nil {
		return nil
	}

	dirs := parseGoWorkUse(string(data))
	if len(dirs) == 0 {
		return nil
	}

	var members []WorkspaceMember
	for _, dir := range dirs {
		m := readGoMember(repoRoot, dir)
		if m != nil {
			members = append(members, *m)
		}
	}
	return members
}

// parseGoWorkUse extracts directory paths from go.work "use" directives.
// Supports both single-line "use ./dir" and block "use ( ... )" syntax.
func parseGoWorkUse(content string) []string {
	var dirs []string
	inBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines.
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		if strings.HasPrefix(trimmed, "use (") || trimmed == "use (" {
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed == ")" {
				inBlock = false
				continue
			}
			dir := strings.TrimSpace(trimmed)
			dir = strings.TrimPrefix(dir, "./")
			if dir != "" {
				dirs = append(dirs, dir)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "use ") {
			dir := strings.TrimSpace(strings.TrimPrefix(trimmed, "use "))
			dir = strings.TrimPrefix(dir, "./")
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

// readGoMember reads a workspace member's go.mod to get the module path.
func readGoMember(repoRoot, relDir string) *WorkspaceMember {
	modPath := filepath.Join(repoRoot, relDir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil
	}

	modulePath := parseGoModModule(string(data))
	if modulePath == "" {
		return nil
	}

	manifestRel := filepath.Join(relDir, "go.mod")
	return &WorkspaceMember{
		Name:         modulePath,
		RootPath:     relDir,
		ManifestPath: manifestRel,
		Ecosystem:    "go",
		PackageNames: []string{modulePath},
	}
}

// parseGoModModule extracts the module path from go.mod content.
func parseGoModModule(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
