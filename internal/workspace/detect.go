package workspace

import (
	"path/filepath"
	"sort"
	"strings"
)

// WorkspaceMember represents a single member (sub-project) in a monorepo workspace.
type WorkspaceMember struct {
	Name         string   // canonical name, e.g. "@myapp/web" or module path
	RootPath     string   // repo-relative directory, e.g. "packages/web"
	ManifestPath string   // repo-relative path to manifest file
	Ecosystem    string   // "npm", "go", "python", "maven", "gradle"
	PackageNames []string // names this member publishes (npm name, Go module path, etc.)
}

// WorkspaceInfo holds detected workspace members and provides lookup helpers.
type WorkspaceInfo struct {
	Members       []WorkspaceMember
	ByPackageName map[string]*WorkspaceMember // lookup by published package name
	membersSorted []WorkspaceMember           // sorted by RootPath descending length for longest-prefix match
	goModPrefixes []string                    // Go module path prefixes for fast import matching
}

// Detect scans a repository root for workspace manifests across all supported ecosystems.
// Returns an empty WorkspaceInfo (not nil) if no workspace is detected.
func Detect(repoRoot string) *WorkspaceInfo {
	var members []WorkspaceMember

	// Try each ecosystem detector. A repo can have multiple workspace types.
	members = append(members, detectJSWorkspaces(repoRoot)...)
	members = append(members, detectGoWorkspaces(repoRoot)...)
	members = append(members, detectPythonWorkspaces(repoRoot)...)
	members = append(members, detectJavaWorkspaces(repoRoot)...)

	return newWorkspaceInfo(members)
}

func newWorkspaceInfo(members []WorkspaceMember) *WorkspaceInfo {
	ws := &WorkspaceInfo{
		Members:       members,
		ByPackageName: make(map[string]*WorkspaceMember, len(members)*2),
	}
	for i := range members {
		m := &members[i]
		for _, name := range m.PackageNames {
			ws.ByPackageName[name] = m
		}
	}
	// Pre-sort by root path length descending for longest-prefix matching.
	ws.membersSorted = make([]WorkspaceMember, len(members))
	copy(ws.membersSorted, members)
	sort.Slice(ws.membersSorted, func(i, j int) bool {
		return len(ws.membersSorted[i].RootPath) > len(ws.membersSorted[j].RootPath)
	})

	// Pre-collect Go module prefixes for fast import matching.
	for i := range members {
		if members[i].Ecosystem != "go" {
			continue
		}
		for _, name := range members[i].PackageNames {
			ws.goModPrefixes = append(ws.goModPrefixes, name)
		}
	}

	return ws
}

// IsEmpty returns true if no workspace members were detected.
func (ws *WorkspaceInfo) IsEmpty() bool {
	return len(ws.Members) == 0
}

// MemberForPath returns the workspace member that owns the given repo-relative file path,
// using longest-prefix matching on directory paths. Returns nil if no member matches.
func (ws *WorkspaceInfo) MemberForPath(repoRelPath string) *WorkspaceMember {
	dir := filepath.Dir(repoRelPath)
	for i := range ws.membersSorted {
		m := &ws.membersSorted[i]
		if dir == m.RootPath || strings.HasPrefix(dir, m.RootPath+"/") {
			return m
		}
	}
	return nil
}

// IsWorkspacePackage returns true if the given package name belongs to a workspace member.
func (ws *WorkspaceInfo) IsWorkspacePackage(name string) bool {
	_, ok := ws.ByPackageName[name]
	return ok
}

// NewForTest creates a WorkspaceInfo from the given members with all indexes built.
// Intended for use in tests outside the workspace package.
func NewForTest(members []WorkspaceMember) *WorkspaceInfo {
	return newWorkspaceInfo(members)
}

// IsGoWorkspaceImport returns true if a Go import path belongs to any workspace member module
// (exact match or sub-package). Uses pre-computed prefixes for O(prefixes) lookup.
func (ws *WorkspaceInfo) IsGoWorkspaceImport(importPath string) bool {
	for _, prefix := range ws.goModPrefixes {
		if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
			return true
		}
	}
	return false
}
