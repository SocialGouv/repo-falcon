package workspace

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func testdataDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "testdata")
}

func TestDetectNPMWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "npm_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected npm workspace members, got empty")
	}
	if len(ws.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ws.Members))
	}

	names := memberNames(ws)
	sort.Strings(names)
	if names[0] != "@myapp/core" || names[1] != "@myapp/web" {
		t.Fatalf("unexpected member names: %v", names)
	}

	// Verify lookup.
	if !ws.IsWorkspacePackage("@myapp/core") {
		t.Error("expected @myapp/core to be a workspace package")
	}
	if ws.IsWorkspacePackage("lodash") {
		t.Error("expected lodash to NOT be a workspace package")
	}
}

func TestDetectPnpmWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "pnpm_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected pnpm workspace members, got empty")
	}
	if len(ws.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ws.Members))
	}

	names := memberNames(ws)
	sort.Strings(names)
	if names[0] != "@pnpm-app/frontend" || names[1] != "@pnpm-app/lib" {
		t.Fatalf("unexpected member names: %v", names)
	}
}

func TestDetectGoWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "go_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected go workspace members, got empty")
	}
	if len(ws.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ws.Members))
	}

	if !ws.IsWorkspacePackage("github.com/example/svc-a") {
		t.Error("expected github.com/example/svc-a to be a workspace package")
	}
	if !ws.IsWorkspacePackage("github.com/example/lib-b") {
		t.Error("expected github.com/example/lib-b to be a workspace package")
	}
}

func TestDetectPythonWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "python_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected python workspace members, got empty")
	}
	if len(ws.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ws.Members))
	}

	if !ws.IsWorkspacePackage("my-core-lib") {
		t.Error("expected my-core-lib to be a workspace package")
	}
	if !ws.IsWorkspacePackage("my-api") {
		t.Error("expected my-api to be a workspace package")
	}
}

func TestDetectMavenWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "maven_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected maven workspace members, got empty")
	}
	if len(ws.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ws.Members))
	}

	if !ws.IsWorkspacePackage("com.example:core") {
		t.Error("expected com.example:core to be a workspace package")
	}
	if !ws.IsWorkspacePackage("com.example:web") {
		t.Error("expected com.example:web to be a workspace package")
	}
}

func TestDetectMavenInheritedGroupID(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "maven_inherited_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected maven workspace members, got empty")
	}
	if len(ws.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(ws.Members))
	}
	// Child pom omits groupId — should inherit "com.inherited" from parent.
	if ws.Members[0].Name != "com.inherited:svc" {
		t.Fatalf("expected com.inherited:svc, got %s", ws.Members[0].Name)
	}
	if !ws.IsWorkspacePackage("com.inherited:svc") {
		t.Error("expected com.inherited:svc to be a workspace package")
	}
}

func TestDetectGradleWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "gradle_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected gradle workspace members, got empty")
	}
	if len(ws.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ws.Members))
	}

	names := memberNames(ws)
	sort.Strings(names)
	if names[0] != ":core" || names[1] != ":web" {
		t.Fatalf("unexpected member names: %v", names)
	}
}

func TestDetectNestedNPMWorkspace(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "nested_npm_workspace"))
	if ws.IsEmpty() {
		t.Fatal("expected nested npm workspace members, got empty")
	}
	if len(ws.Members) != 1 {
		t.Fatalf("expected 1 member, got %d: %v", len(ws.Members), memberNames(ws))
	}
	if ws.Members[0].Name != "@nested/core" {
		t.Fatalf("expected @nested/core, got %s", ws.Members[0].Name)
	}
}

func TestDetectEmptyRepo(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "empty_repo"))
	if !ws.IsEmpty() {
		t.Fatalf("expected empty workspace, got %d members", len(ws.Members))
	}
}

func TestMemberForPath(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "npm_workspace"))

	m := ws.MemberForPath("packages/core/src/index.ts")
	if m == nil {
		t.Fatal("expected member for packages/core/src/index.ts")
	}
	if m.Name != "@myapp/core" {
		t.Fatalf("expected @myapp/core, got %s", m.Name)
	}

	m = ws.MemberForPath("packages/web/app.tsx")
	if m == nil {
		t.Fatal("expected member for packages/web/app.tsx")
	}
	if m.Name != "@myapp/web" {
		t.Fatalf("expected @myapp/web, got %s", m.Name)
	}

	// Root-level file should not match any member.
	m = ws.MemberForPath("README.md")
	if m != nil {
		t.Fatalf("expected nil for root-level file, got %s", m.Name)
	}
}

func TestParseUVWorkspaceMembersMultiline(t *testing.T) {
	content := `[project]
name = "my-monorepo"

[tool.uv.workspace]
members = [
  "packages/*",
  "libs/*",
]

[tool.other]
key = "value"
`
	patterns := parseUVWorkspaceMembers(content)
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d: %v", len(patterns), patterns)
	}
	if patterns[0] != "packages/*" || patterns[1] != "libs/*" {
		t.Fatalf("unexpected patterns: %v", patterns)
	}
}

func TestParseUVWorkspaceMembersSingleLine(t *testing.T) {
	content := `[tool.uv.workspace]
members = ["packages/*"]
`
	patterns := parseUVWorkspaceMembers(content)
	if len(patterns) != 1 || patterns[0] != "packages/*" {
		t.Fatalf("unexpected patterns: %v", patterns)
	}
}

func TestIsGoWorkspaceImport(t *testing.T) {
	ws := Detect(filepath.Join(testdataDir(), "go_workspace"))

	tests := []struct {
		importPath string
		want       bool
	}{
		{"github.com/example/svc-a", true},
		{"github.com/example/svc-a/internal/handler", true},
		{"github.com/example/lib-b", true},
		{"github.com/example/lib-b/pkg/util", true},
		{"github.com/example/other", false},
		{"github.com/example/svc-ab", false}, // must not match svc-a prefix
		{"fmt", false},
	}
	for _, tt := range tests {
		got := ws.IsGoWorkspaceImport(tt.importPath)
		if got != tt.want {
			t.Errorf("IsGoWorkspaceImport(%q) = %v, want %v", tt.importPath, got, tt.want)
		}
	}
}

func TestIsGoWorkspaceImportEmpty(t *testing.T) {
	ws := newWorkspaceInfo(nil)
	if ws.IsGoWorkspaceImport("anything") {
		t.Error("expected false for empty workspace")
	}
}

func TestParseGoWorkUseWithInlineComments(t *testing.T) {
	content := `go 1.21

use (
	./svc-a // main service
	./lib-b // shared library
)
`
	dirs := parseGoWorkUse(content)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
	}
	if dirs[0] != "svc-a" || dirs[1] != "lib-b" {
		t.Fatalf("unexpected dirs: %v", dirs)
	}
}

func TestParseGoWorkUseSingleLineWithComment(t *testing.T) {
	content := `go 1.21

use ./mymod // workspace member
`
	dirs := parseGoWorkUse(content)
	if len(dirs) != 1 || dirs[0] != "mymod" {
		t.Fatalf("expected [mymod], got %v", dirs)
	}
}

func TestParseGradleIncludesMultiline(t *testing.T) {
	content := `rootProject.name = "my-project"

include(
  ":core",
  ":web"
)
`
	projects := parseGradleIncludes(content)
	sort.Strings(projects)
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d: %v", len(projects), projects)
	}
	if projects[0] != ":core" || projects[1] != ":web" {
		t.Fatalf("unexpected projects: %v", projects)
	}
}

func TestExpandDoubleStarGlob(t *testing.T) {
	// nested_npm_workspace has packages/**/core structure.
	root := filepath.Join(testdataDir(), "nested_npm_workspace")
	dirs := expandDoubleStarGlob(root, "packages/**")
	if len(dirs) == 0 {
		t.Fatal("expected at least one directory from packages/**")
	}

	// Should find "packages/group" and "packages/group/core" (the nested structure).
	found := map[string]bool{}
	for _, d := range dirs {
		found[d] = true
	}
	if !found["packages/group"] {
		t.Errorf("expected packages/group in results, got %v", dirs)
	}
	if !found["packages/group/core"] {
		t.Errorf("expected packages/group/core in results, got %v", dirs)
	}
}

func TestExpandDoubleStarGlobSkipsHiddenAndNodeModules(t *testing.T) {
	// expandDoubleStarGlob should skip .hidden dirs and node_modules.
	// We test this by checking no results contain these patterns.
	root := filepath.Join(testdataDir(), "nested_npm_workspace")
	dirs := expandDoubleStarGlob(root, "packages/**")
	for _, d := range dirs {
		base := filepath.Base(d)
		if strings.HasPrefix(base, ".") {
			t.Errorf("found hidden dir in results: %s", d)
		}
		if base == "node_modules" {
			t.Errorf("found node_modules in results: %s", d)
		}
	}
}

func memberNames(ws *WorkspaceInfo) []string {
	names := make([]string, len(ws.Members))
	for i, m := range ws.Members {
		names[i] = m.Name
	}
	return names
}
