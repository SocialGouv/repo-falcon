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

	var members []WorkspaceMember
	for _, mod := range modules {
		m := readMavenMember(repoRoot, mod)
		if m != nil {
			members = append(members, *m)
		}
	}
	return members
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
func readMavenMember(repoRoot, relDir string) *WorkspaceMember {
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

	name := pom.ArtifactID
	if pom.GroupID != "" {
		name = pom.GroupID + ":" + pom.ArtifactID
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
func parseGradleIncludes(content string) []string {
	var projects []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}

		// Match include("project1", "project2") or include ':project1', ':project2'
		matches := gradleIncludeRe.FindAllStringSubmatch(trimmed, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			// Extract quoted strings from the argument list.
			quoted := gradleQuoteRe.FindAllStringSubmatch(m[1], -1)
			for _, q := range quoted {
				if len(q) >= 2 {
					projects = append(projects, q[1])
				}
			}
		}

		// Also handle Groovy-style: include ':project1', ':project2' (no parentheses).
		if strings.HasPrefix(trimmed, "include ") && !strings.Contains(trimmed, "(") {
			rest := strings.TrimPrefix(trimmed, "include ")
			quoted := gradleQuoteRe.FindAllStringSubmatch(rest, -1)
			for _, q := range quoted {
				if len(q) >= 2 {
					projects = append(projects, q[1])
				}
			}
		}
	}
	return projects
}
