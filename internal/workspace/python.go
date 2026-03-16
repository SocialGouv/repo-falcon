package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// detectPythonWorkspaces detects Python workspaces (uv workspaces, multiple pyproject.toml).
func detectPythonWorkspaces(repoRoot string) []WorkspaceMember {
	// Try uv workspace format in pyproject.toml.
	if members := detectUVWorkspace(repoRoot); len(members) > 0 {
		return members
	}
	return nil
}

// detectUVWorkspace parses pyproject.toml for [tool.uv.workspace] members.
// Uses line-based parsing to avoid a TOML dependency.
//
// Expected format:
//
//	[tool.uv.workspace]
//	members = ["packages/*", "libs/*"]
func detectUVWorkspace(repoRoot string) []WorkspaceMember {
	data, err := os.ReadFile(filepath.Join(repoRoot, "pyproject.toml"))
	if err != nil {
		return nil
	}

	patterns := parseUVWorkspaceMembers(string(data))
	if len(patterns) == 0 {
		return nil
	}

	var members []WorkspaceMember
	for _, pattern := range patterns {
		dirs := expandGlob(repoRoot, pattern)
		for _, dir := range dirs {
			m := readPythonMember(repoRoot, dir)
			if m != nil {
				members = append(members, *m)
			}
		}
	}
	return members
}

// parseUVWorkspaceMembers extracts member patterns from pyproject.toml content.
// Looks for: members = ["packages/*", "libs/*"] under [tool.uv.workspace].
func parseUVWorkspaceMembers(content string) []string {
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Detect section headers.
		if strings.HasPrefix(trimmed, "[") {
			if trimmed == "[tool.uv.workspace]" {
				inSection = true
			} else {
				inSection = false
			}
			continue
		}

		if !inSection {
			continue
		}

		// Look for members = [...]
		if strings.HasPrefix(trimmed, "members") {
			idx := strings.Index(trimmed, "=")
			if idx < 0 {
				continue
			}
			val := strings.TrimSpace(trimmed[idx+1:])
			return parseTOMLStringArray(val)
		}
	}
	return nil
}

// parseTOMLStringArray parses a simple TOML array like ["a", "b"].
func parseTOMLStringArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	s = s[1 : len(s)-1]
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"'`)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// readPythonMember reads a workspace member's pyproject.toml to get the project name.
func readPythonMember(repoRoot, relDir string) *WorkspaceMember {
	pyprojectPath := filepath.Join(repoRoot, relDir, "pyproject.toml")
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return nil
	}

	name := parsePyProjectName(string(data))
	if name == "" {
		// Fallback to directory name.
		name = filepath.Base(relDir)
	}

	manifestRel := filepath.Join(relDir, "pyproject.toml")
	return &WorkspaceMember{
		Name:         name,
		RootPath:     relDir,
		ManifestPath: manifestRel,
		Ecosystem:    "python",
		PackageNames: []string{name},
	}
}

// parsePyProjectName extracts the project name from pyproject.toml.
// Looks for: name = "mypackage" under [project].
func parsePyProjectName(content string) string {
	inProject := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") {
			if trimmed == "[project]" {
				inProject = true
			} else {
				inProject = false
			}
			continue
		}

		if !inProject {
			continue
		}

		if strings.HasPrefix(trimmed, "name") {
			idx := strings.Index(trimmed, "=")
			if idx < 0 {
				continue
			}
			val := strings.TrimSpace(trimmed[idx+1:])
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}
