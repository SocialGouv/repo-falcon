package cli

import (
	"testing"

	"repofalcon/internal/workspace"
)

func TestWsPackageRowForWithWorkspace(t *testing.T) {
	ws := workspace.NewForTest([]workspace.WorkspaceMember{
		{
			Name:         "@myapp/core",
			RootPath:     "packages/core",
			ManifestPath: "packages/core/package.json",
			Ecosystem:    "npm",
			PackageNames: []string{"@myapp/core"},
		},
	})

	// Package name matches a workspace member.
	row := wsPackageRowFor("js", "@myapp/core", true, ws, "")
	if row.Scope != "@myapp/core" {
		t.Errorf("expected Scope=@myapp/core, got %q", row.Scope)
	}
	if row.RootPath == nil || *row.RootPath != "packages/core" {
		t.Errorf("expected RootPath=packages/core, got %v", row.RootPath)
	}
	if row.ManifestPath == nil || *row.ManifestPath != "packages/core/package.json" {
		t.Errorf("expected ManifestPath=packages/core/package.json, got %v", row.ManifestPath)
	}

	// External package should not get workspace metadata.
	row = wsPackageRowFor("js", "lodash", false, ws, "")
	if row.Scope != "" {
		t.Errorf("expected empty Scope for lodash, got %q", row.Scope)
	}
	if row.RootPath != nil {
		t.Errorf("expected nil RootPath for lodash, got %v", row.RootPath)
	}
}

func TestWsPackageRowForByPath(t *testing.T) {
	ws := workspace.NewForTest([]workspace.WorkspaceMember{
		{
			Name:         "@myapp/web",
			RootPath:     "packages/web",
			ManifestPath: "packages/web/package.json",
			Ecosystem:    "npm",
			PackageNames: []string{"@myapp/web"},
		},
	})

	// Lookup by file path (directory-based package name won't match ByPackageName).
	row := wsPackageRowFor("js", "packages/web", true, ws, "packages/web/src/index.ts")
	if row.Scope != "@myapp/web" {
		t.Errorf("expected Scope=@myapp/web, got %q", row.Scope)
	}
}

func TestWsPackageRowForEmptyWorkspace(t *testing.T) {
	ws := workspace.NewForTest(nil)

	row := wsPackageRowFor("go", "mymod/pkg", true, ws, "pkg/main.go")
	if row.Scope != "" {
		t.Errorf("expected empty Scope for empty workspace, got %q", row.Scope)
	}
	if row.Name != "mymod/pkg" {
		t.Errorf("expected Name=mymod/pkg, got %q", row.Name)
	}
	if !row.IsInternal {
		t.Error("expected IsInternal=true")
	}
}
