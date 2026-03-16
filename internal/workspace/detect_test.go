package workspace

import (
	"path/filepath"
	"runtime"
	"sort"
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

func memberNames(ws *WorkspaceInfo) []string {
	names := make([]string, len(ws.Members))
	for i, m := range ws.Members {
		names[i] = m.Name
	}
	return names
}
