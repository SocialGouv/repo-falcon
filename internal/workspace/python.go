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
// Supports both single-line and multiline array formats.
func parseUVWorkspaceMembers(content string) []string {
	inSection := false
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

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

			// If the array is complete on one line, parse it directly.
			if strings.Contains(val, "]") {
				return parseTOMLStringArray(val)
			}

			// Multiline array: collect lines until we find the closing ']'.
			var buf strings.Builder
			buf.WriteString(val)
			for i++; i < len(lines); i++ {
				line := strings.TrimSpace(lines[i])
				// Stop at a new section header.
				if strings.HasPrefix(line, "[") {
					break
				}
				buf.WriteString(" ")
				buf.WriteString(line)
				if strings.Contains(line, "]") {
					break
				}
			}
			return parseTOMLStringArray(buf.String())
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
