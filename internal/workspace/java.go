package workspace

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectJavaWorkspaces detects Maven multi-module or Gradle multi-project setups.
func detectJavaWorkspaces(repoRoot string) []WorkspaceMember {
	if members := detectMavenModules(repoRoot); len(members) > 0 {
		return members
	}
	if members := detectGradleProjects(repoRoot); len(members) > 0 {
		return members
	}
	return nil
}

// detectMavenModules parses root pom.xml for <modules>.
func detectMavenModules(repoRoot string) []WorkspaceMember {
	data, err := os.ReadFile(filepath.Join(repoRoot, "pom.xml"))
	if err != nil {
		return nil
	}

	modules := parseMavenModules(data)
	if len(modules) == 0 {
		return nil
	}

	parentGroupID := parseMavenGroupID(data)

	var members []WorkspaceMember
	for _, mod := range modules {
		m := readMavenMember(repoRoot, mod, parentGroupID)
		if m != nil {
			members = append(members, *m)
		}
	}
	return members
}

// parseMavenGroupID extracts the groupId from a pom.xml.
func parseMavenGroupID(data []byte) string {
	var pom struct {
		GroupID string `xml:"groupId"`
	}
	if err := xml.Unmarshal(data, &pom); err != nil {
		return ""
	}
	return pom.GroupID
}

// parseMavenModules extracts module names from a pom.xml.
func parseMavenModules(data []byte) []string {
	var pom struct {
		Modules struct {
			Module []string `xml:"module"`
		} `xml:"modules"`
	}
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil
	}
	return pom.Modules.Module
}

// readMavenMember reads a module's pom.xml to get groupId:artifactId.
// parentGroupID is used as fallback when the child pom omits groupId (Maven inheritance).
func readMavenMember(repoRoot, relDir, parentGroupID string) *WorkspaceMember {
	pomPath := filepath.Join(repoRoot, relDir, "pom.xml")
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return nil
	}

	var pom struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
	}
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil
	}

	// Inherit groupId from parent if not specified in child.
	groupID := pom.GroupID
	if groupID == "" {
		groupID = parentGroupID
	}

	name := pom.ArtifactID
	if groupID != "" {
		name = groupID + ":" + pom.ArtifactID
	}
	if name == "" {
		name = filepath.Base(relDir)
	}

	manifestRel := filepath.Join(relDir, "pom.xml")
	return &WorkspaceMember{
		Name:         name,
		RootPath:     relDir,
		ManifestPath: manifestRel,
		Ecosystem:    "maven",
		PackageNames: []string{name},
	}
}

var gradleIncludeRe = regexp.MustCompile(`include\s*\(\s*(.+?)\s*\)`)
var gradleQuoteRe = regexp.MustCompile(`["']([^"']+)["']`)

// detectGradleProjects parses settings.gradle(.kts) for include directives.
func detectGradleProjects(repoRoot string) []WorkspaceMember {
	var data []byte
	var err error
	for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
		data, err = os.ReadFile(filepath.Join(repoRoot, name))
		if err == nil {
			break
		}
	}
	if data == nil {
		return nil
	}

	projects := parseGradleIncludes(string(data))
	if len(projects) == 0 {
		return nil
	}

	var members []WorkspaceMember
	for _, proj := range projects {
		// Gradle project ":core:utils" maps to directory "core/utils".
		dir := strings.ReplaceAll(strings.TrimPrefix(proj, ":"), ":", "/")
		if dir == "" {
			continue
		}
		// Verify directory exists.
		if info, err := os.Stat(filepath.Join(repoRoot, dir)); err != nil || !info.IsDir() {
			continue
		}

		manifestRel := ""
		for _, name := range []string{"build.gradle.kts", "build.gradle"} {
			if _, err := os.Stat(filepath.Join(repoRoot, dir, name)); err == nil {
				manifestRel = filepath.Join(dir, name)
				break
			}
		}

		members = append(members, WorkspaceMember{
			Name:         proj,
			RootPath:     dir,
			ManifestPath: manifestRel,
			Ecosystem:    "gradle",
			PackageNames: []string{proj},
		})
	}
	return members
}

// parseGradleIncludes extracts project names from Gradle settings file.
// Supports single-line and multiline include directives.
func parseGradleIncludes(content string) []string {
	var projects []string
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}

		// Single-line: include("project1", "project2") — regex matches closing paren.
		matches := gradleIncludeRe.FindAllStringSubmatch(trimmed, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			projects = append(projects, extractQuoted(m[1])...)
		}

		// Multiline: include( on this line, closing ) on a later line.
		if len(matches) == 0 && strings.Contains(trimmed, "include") && strings.Contains(trimmed, "(") && !strings.Contains(trimmed, ")") {
			var buf strings.Builder
			buf.WriteString(trimmed)
			for i++; i < len(lines); i++ {
				line := strings.TrimSpace(lines[i])
				buf.WriteString(" ")
				buf.WriteString(line)
				if strings.Contains(line, ")") {
					break
				}
			}
			joined := buf.String()
			m := gradleIncludeRe.FindStringSubmatch(joined)
			if len(m) >= 2 {
				projects = append(projects, extractQuoted(m[1])...)
			}
		}

		// Groovy-style: include ':project1', ':project2' (no parentheses).
		if !strings.Contains(trimmed, "(") && strings.HasPrefix(trimmed, "include ") {
			rest := strings.TrimPrefix(trimmed, "include ")
			projects = append(projects, extractQuoted(rest)...)
		}
	}
	return projects
}

// extractQuoted returns all quoted strings from s.
func extractQuoted(s string) []string {
	matches := gradleQuoteRe.FindAllStringSubmatch(s, -1)
	var result []string
	for _, q := range matches {
		if len(q) >= 2 {
			result = append(result, q[1])
		}
	}
	return result
}
