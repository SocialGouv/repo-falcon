package mcp

import (
	"strings"
	"testing"

	"repofalcon/internal/artifacts"
)

func strPtr(s string) *string { return &s }

func TestWorkspaceInfoOverview(t *testing.T) {
	g := &GraphIndex{
		PkgsByScope: map[string][]artifacts.PackageRow{
			"@myapp/core": {
				{PackageID: "js:@myapp/core", Ecosystem: "js", Scope: "@myapp/core", Name: "@myapp/core", IsInternal: true, RootPath: strPtr("packages/core")},
			},
			"@myapp/web": {
				{PackageID: "js:@myapp/web", Ecosystem: "js", Scope: "@myapp/web", Name: "@myapp/web", IsInternal: true, RootPath: strPtr("packages/web")},
			},
		},
		PkgFiles: map[string][]string{
			"js:@myapp/core": {"file:packages/core/index.ts"},
			"js:@myapp/web":  {"file:packages/web/app.ts", "file:packages/web/main.ts"},
		},
	}

	result := g.WorkspaceInfo("")
	if !strings.Contains(result, "Workspace Overview") {
		t.Errorf("expected 'Workspace Overview' header, got:\n%s", result)
	}
	if !strings.Contains(result, "Members: 2") {
		t.Errorf("expected 'Members: 2', got:\n%s", result)
	}
	if !strings.Contains(result, "@myapp/core") || !strings.Contains(result, "@myapp/web") {
		t.Errorf("expected both members listed, got:\n%s", result)
	}
}

func TestWorkspaceInfoMemberDetail(t *testing.T) {
	g := &GraphIndex{
		PkgsByScope: map[string][]artifacts.PackageRow{
			"@myapp/core": {
				{PackageID: "js:@myapp/core", Ecosystem: "js", Scope: "@myapp/core", Name: "@myapp/core", IsInternal: true, RootPath: strPtr("packages/core"), ManifestPath: strPtr("packages/core/package.json")},
			},
		},
		PkgFiles: map[string][]string{
			"js:@myapp/core": {"file:packages/core/index.ts"},
		},
		PkgByID:     map[string]artifacts.PackageRow{},
		FileImports: map[string][]string{},
	}

	result := g.WorkspaceInfo("@myapp/core")
	if !strings.Contains(result, "Workspace Member: @myapp/core") {
		t.Errorf("expected member header, got:\n%s", result)
	}
	if !strings.Contains(result, "Root: packages/core") {
		t.Errorf("expected root path, got:\n%s", result)
	}
	if !strings.Contains(result, "Manifest: packages/core/package.json") {
		t.Errorf("expected manifest path, got:\n%s", result)
	}
}

func TestWorkspaceInfoMemberNotFound(t *testing.T) {
	g := &GraphIndex{
		PkgsByScope: map[string][]artifacts.PackageRow{
			"@myapp/core": {
				{PackageID: "js:@myapp/core", Ecosystem: "js", Scope: "@myapp/core", Name: "@myapp/core", IsInternal: true},
			},
		},
	}

	result := g.WorkspaceInfo("nonexistent")
	if !strings.Contains(result, "Workspace member not found") {
		t.Errorf("expected 'not found' message, got:\n%s", result)
	}
	if !strings.Contains(result, "@myapp/core") {
		t.Errorf("expected available members listed, got:\n%s", result)
	}
}

func TestWorkspaceInfoEmpty(t *testing.T) {
	g := &GraphIndex{
		PkgsByScope: map[string][]artifacts.PackageRow{},
	}

	result := g.WorkspaceInfo("")
	if !strings.Contains(result, "No workspace/monorepo") {
		t.Errorf("expected no-workspace message, got:\n%s", result)
	}
}

func TestWorkspaceInfoCrossMemberDeps(t *testing.T) {
	g := &GraphIndex{
		PkgsByScope: map[string][]artifacts.PackageRow{
			"@myapp/web": {
				{PackageID: "js:@myapp/web", Ecosystem: "js", Scope: "@myapp/web", Name: "@myapp/web", IsInternal: true, RootPath: strPtr("packages/web")},
			},
			"@myapp/core": {
				{PackageID: "js:@myapp/core", Ecosystem: "js", Scope: "@myapp/core", Name: "@myapp/core", IsInternal: true, RootPath: strPtr("packages/core")},
			},
		},
		PkgFiles: map[string][]string{
			"js:@myapp/web": {"file:packages/web/app.ts"},
		},
		FileImports: map[string][]string{
			"file:packages/web/app.ts": {"js:@myapp/core"},
		},
		PkgByID: map[string]artifacts.PackageRow{
			"js:@myapp/core": {PackageID: "js:@myapp/core", Ecosystem: "js", Scope: "@myapp/core", Name: "@myapp/core", IsInternal: true},
		},
	}

	result := g.WorkspaceInfo("@myapp/web")
	if !strings.Contains(result, "Cross-member Dependencies") {
		t.Errorf("expected cross-member section, got:\n%s", result)
	}
	if !strings.Contains(result, "→ @myapp/core") {
		t.Errorf("expected dependency on @myapp/core, got:\n%s", result)
	}
}
