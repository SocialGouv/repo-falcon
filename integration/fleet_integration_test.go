package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"repofalcon/internal/fleet"
	"repofalcon/internal/mcp"
)

// TestFleet_Sync_IndexesAllRepos verifies that "falcon fleet sync" indexes
// every repo listed in a manifest and produces valid artifacts for each.
func TestFleet_Sync_IndexesAllRepos(t *testing.T) {
	bin := falconBin(t)

	// Use two existing fixtures as separate "repos" in a fleet.
	root := repoRootDir(t)
	repo1 := filepath.Join(root, "testdata", "tinyrepo_go_js")
	repo2 := filepath.Join(root, "testdata", "tinyrepo_go_py")

	// Create fleet manifest pointing to the two fixtures.
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "fleet.json")
	writeFleetManifest(t, manifestPath, []fleet.RepoEntry{
		{Path: repo1, Name: "go-js"},
		{Path: repo2, Name: "go-py"},
	})

	// Run fleet sync.
	runFalcon(t, bin, "fleet", "sync", "--manifest", manifestPath)

	// Verify artifacts exist for both repos.
	for _, entry := range []struct {
		repoPath string
		name     string
	}{
		{repo1, "go-js"},
		{repo2, "go-py"},
	} {
		artDir := filepath.Join(entry.repoPath, ".falcon", "artifacts")
		for _, table := range []string{"files.parquet", "packages.parquet", "symbols.parquet", "edges.parquet", "findings.parquet", "nodes.parquet", "metadata.json"} {
			mustExist(t, filepath.Join(artDir, table))
		}
	}
}

// TestFleet_Sync_WithReposFlag verifies --repos flag overrides manifest.
func TestFleet_Sync_WithReposFlag(t *testing.T) {
	bin := falconBin(t)

	root := repoRootDir(t)
	repo1 := filepath.Join(root, "testdata", "tinyrepo_go_js")

	// Run fleet sync with --repos flag (no manifest file needed).
	runFalcon(t, bin, "fleet", "sync", "--repos", repo1)

	artDir := filepath.Join(repo1, ".falcon", "artifacts")
	mustExist(t, filepath.Join(artDir, "files.parquet"))
	mustExist(t, filepath.Join(artDir, "symbols.parquet"))
}

// TestFleet_LoadFleetGraph verifies that LoadFleetGraph loads multiple
// indexed repos into a queryable FleetGraph.
func TestFleet_LoadFleetGraph(t *testing.T) {
	bin := falconBin(t)

	root := repoRootDir(t)
	repo1 := filepath.Join(root, "testdata", "tinyrepo_go_js")
	repo2 := filepath.Join(root, "testdata", "tinyrepo_go_py")

	// Ensure both repos are indexed.
	runFalcon(t, bin, "fleet", "sync", "--repos", repo1+","+repo2)

	ctx := context.Background()
	m := &fleet.Manifest{
		Version: "1",
		Repos: []fleet.RepoEntry{
			{Path: repo1, Name: "go-js"},
			{Path: repo2, Name: "go-py"},
		},
	}

	fg, err := fleet.LoadFleetGraph(ctx, m)
	if err != nil {
		t.Fatalf("LoadFleetGraph: %v", err)
	}

	if len(fg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(fg.Repos))
	}

	// Verify both repos are loaded by name.
	if _, ok := fg.ByName["go-js"]; !ok {
		t.Fatal("expected go-js repo in ByName")
	}
	if _, ok := fg.ByName["go-py"]; !ok {
		t.Fatal("expected go-py repo in ByName")
	}

	// Verify each repo has files and symbols.
	for _, rg := range fg.Repos {
		if len(rg.Graph.Files) == 0 {
			t.Fatalf("repo %s has no files", rg.Name)
		}
		if len(rg.Graph.Symbols) == 0 {
			t.Fatalf("repo %s has no symbols", rg.Name)
		}
	}
}

// TestFleet_FleetOverview verifies the overview output lists all repos.
func TestFleet_FleetOverview(t *testing.T) {
	fg := loadTestFleetGraph(t)

	overview := fg.FleetOverview()

	if !strings.Contains(overview, "go-js") {
		t.Fatal("overview missing go-js repo")
	}
	if !strings.Contains(overview, "go-py") {
		t.Fatal("overview missing go-py repo")
	}
	if !strings.Contains(overview, "Total repos: 2") {
		t.Fatal("overview missing total repos count")
	}
}

// TestFleet_SearchAll verifies cross-repo search returns results from both repos.
func TestFleet_SearchAll(t *testing.T) {
	fg := loadTestFleetGraph(t)

	// Search for .go files (should be in both repos).
	result := fg.SearchAll(".go", "file")
	if !strings.Contains(result, "go-js") || !strings.Contains(result, "go-py") {
		t.Fatalf("expected search to find .go files in both repos, got:\n%s", result)
	}
}

// TestFleet_FindReposByDependency verifies dependency search across repos.
func TestFleet_FindReposByDependency(t *testing.T) {
	fg := loadTestFleetGraph(t)

	// Both tinyrepos have a go.mod, so they should have some Go dependencies.
	// Search for something generic like "fmt" which Go files import.
	result := fg.FindReposByDependency("fmt")
	if !strings.Contains(result, "Repos depending on: fmt") {
		t.Fatalf("unexpected result: %s", result)
	}
}

// TestFleet_CommonDependencies verifies shared dependency detection.
func TestFleet_CommonDependencies(t *testing.T) {
	fg := loadTestFleetGraph(t)

	result := fg.CommonDependencies()
	if !strings.Contains(result, "Common Dependencies") {
		t.Fatalf("unexpected result: %s", result)
	}
}

// TestFleet_RepoArchitecture verifies scoped architecture query.
func TestFleet_RepoArchitecture(t *testing.T) {
	fg := loadTestFleetGraph(t)

	arch, err := fg.RepoArchitecture("go-js")
	if err != nil {
		t.Fatalf("RepoArchitecture: %v", err)
	}
	if !strings.Contains(arch, "Architecture Overview") {
		t.Fatalf("unexpected result: %s", arch)
	}

	// Non-existent repo should error.
	_, err = fg.RepoArchitecture("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent repo")
	}
}

// TestFleet_SymbolLookup_CrossRepo verifies symbol lookup across all repos.
func TestFleet_SymbolLookup_CrossRepo(t *testing.T) {
	fg := loadTestFleetGraph(t)

	// Search for a symbol that likely exists in the Go fixture.
	// tinyrepo_go_js has greeter.go which should define something.
	result := fg.SymbolLookup("", "Greet", "")
	// We just verify it doesn't crash and returns something meaningful.
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestFleet_SymbolLookup_ScopedToRepo verifies scoped symbol lookup.
func TestFleet_SymbolLookup_ScopedToRepo(t *testing.T) {
	fg := loadTestFleetGraph(t)

	result := fg.SymbolLookup("go-js", "Greet", "")
	if result == "" {
		t.Fatal("expected non-empty result for scoped lookup")
	}

	// Scoped to wrong repo should show no results.
	result = fg.SymbolLookup("nonexistent", "Greet", "")
	if !strings.Contains(result, "Repo not found") {
		t.Fatalf("expected repo not found error, got: %s", result)
	}
}

// TestFleet_MCPServer_ToolsList verifies the fleet MCP server exposes the right tools.
func TestFleet_MCPServer_ToolsList(t *testing.T) {
	fg := loadTestFleetGraph(t)
	srv := fleet.NewFleetServer(fg)

	// Test through a pipe.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rr, rw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(pr, rw)
	}()

	// Send initialize request.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	if _, err := pw.WriteString(initReq); err != nil {
		t.Fatal(err)
	}

	// Send tools/list request.
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	if _, err := pw.WriteString(listReq); err != nil {
		t.Fatal(err)
	}

	// Close writer to let server finish.
	pw.Close()
	<-done

	// Read responses.
	buf := make([]byte, 64*1024)
	n, _ := rr.Read(buf)
	rr.Close()
	rw.Close()

	output := string(buf[:n])

	// Verify all fleet tools are present.
	expectedTools := []string{
		"fleet_overview",
		"fleet_search",
		"fleet_repo_architecture",
		"fleet_file_context",
		"fleet_symbol_lookup",
		"fleet_find_repos_by_dependency",
		"fleet_common_dependencies",
	}
	for _, tool := range expectedTools {
		if !strings.Contains(output, tool) {
			t.Errorf("expected tool %s in MCP response, got:\n%s", tool, output)
		}
	}
}

// TestFleet_MCPServer_FleetOverviewCall verifies a fleet_overview tool call via MCP.
func TestFleet_MCPServer_FleetOverviewCall(t *testing.T) {
	fg := loadTestFleetGraph(t)
	srv := fleet.NewFleetServer(fg)

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rr, rw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(pr, rw)
	}()

	// Send initialize + tool call.
	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"fleet_overview","arguments":{}}}` + "\n"
	if _, err := pw.WriteString(requests); err != nil {
		t.Fatal(err)
	}
	pw.Close()
	<-done

	buf := make([]byte, 64*1024)
	n, _ := rr.Read(buf)
	rr.Close()
	rw.Close()

	output := string(buf[:n])
	if !strings.Contains(output, "go-js") || !strings.Contains(output, "go-py") {
		t.Fatalf("fleet_overview call should mention both repos, got:\n%s", output)
	}
}

// TestFleet_Manifest_LoadAndValidate tests manifest parsing and validation.
func TestFleet_Manifest_LoadAndValidate(t *testing.T) {
	tmp := t.TempDir()

	// Valid manifest.
	path := filepath.Join(tmp, "fleet.json")
	writeFleetManifest(t, path, []fleet.RepoEntry{
		{Path: tmp, Name: "test"},
	})
	m, err := fleet.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(m.Repos))
	}
	if m.Repos[0].EffectiveName() != "test" {
		t.Fatalf("expected name 'test', got %q", m.Repos[0].EffectiveName())
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Invalid path.
	badPath := filepath.Join(tmp, "bad.json")
	writeFleetManifest(t, badPath, []fleet.RepoEntry{
		{Path: "/nonexistent/path/xyz"},
	})
	m2, err := fleet.LoadManifest(badPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if err := m2.Validate(); err == nil {
		t.Fatal("expected validation error for nonexistent path")
	}

	// Empty repos.
	emptyPath := filepath.Join(tmp, "empty.json")
	writeFleetManifest(t, emptyPath, nil)
	m3, err := fleet.LoadManifest(emptyPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if err := m3.Validate(); err == nil {
		t.Fatal("expected validation error for empty repos")
	}
}

// TestFleet_ManifestFromPaths tests building a manifest from CLI paths.
func TestFleet_ManifestFromPaths(t *testing.T) {
	m := fleet.ManifestFromPaths([]string{"/a/b", "/c/d"})
	if len(m.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(m.Repos))
	}
	if m.Repos[0].EffectiveName() != "b" {
		t.Fatalf("expected name 'b', got %q", m.Repos[0].EffectiveName())
	}
	if m.Repos[1].EffectiveName() != "d" {
		t.Fatalf("expected name 'd', got %q", m.Repos[1].EffectiveName())
	}
}

// TestFleet_RepoEntry_Defaults tests RepoEntry default behavior.
func TestFleet_RepoEntry_Defaults(t *testing.T) {
	e := fleet.RepoEntry{Path: "/home/user/my-app"}

	if e.EffectiveName() != "my-app" {
		t.Fatalf("expected 'my-app', got %q", e.EffectiveName())
	}

	expected := "/home/user/my-app/.falcon/artifacts"
	if e.EffectiveArtifactsDir() != expected {
		t.Fatalf("expected %q, got %q", expected, e.EffectiveArtifactsDir())
	}

	// With overrides.
	e2 := fleet.RepoEntry{Path: "/home/user/my-app", Name: "custom", Artifacts: "/tmp/arts"}
	if e2.EffectiveName() != "custom" {
		t.Fatalf("expected 'custom', got %q", e2.EffectiveName())
	}
	if e2.EffectiveArtifactsDir() != "/tmp/arts" {
		t.Fatalf("expected '/tmp/arts', got %q", e2.EffectiveArtifactsDir())
	}
}

// TestFleet_RepoFileContext verifies file context scoped to a repo.
func TestFleet_RepoFileContext(t *testing.T) {
	fg := loadTestFleetGraph(t)

	// tinyrepo_go_js has pkg/greeter/greeter.go.
	result, err := fg.RepoFileContext("go-js", "pkg/greeter/greeter.go")
	if err != nil {
		t.Fatalf("RepoFileContext: %v", err)
	}
	if !strings.Contains(result, "greeter.go") {
		t.Fatalf("expected file context to contain greeter.go, got:\n%s", result)
	}

	// Non-existent repo should error.
	_, err = fg.RepoFileContext("nonexistent", "main.go")
	if err == nil {
		t.Fatal("expected error for nonexistent repo")
	}

	// Non-existent file returns "File not found" (not an error).
	result, err = fg.RepoFileContext("go-js", "does/not/exist.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "File not found") {
		t.Fatalf("expected 'File not found' for missing file, got:\n%s", result)
	}
}

// TestFleet_MCPServer_SearchCall verifies fleet_search via MCP JSON-RPC.
func TestFleet_MCPServer_SearchCall(t *testing.T) {
	fg := loadTestFleetGraph(t)
	output := mcpToolCall(t, fg, "fleet_search", `{"query":".go","scope":"file"}`)

	if !strings.Contains(output, "go-js") {
		t.Fatalf("fleet_search should return results from go-js, got:\n%s", output)
	}
}

// TestFleet_MCPServer_FindReposByDepCall verifies fleet_find_repos_by_dependency via MCP.
func TestFleet_MCPServer_FindReposByDepCall(t *testing.T) {
	fg := loadTestFleetGraph(t)
	output := mcpToolCall(t, fg, "fleet_find_repos_by_dependency", `{"dependency":"fmt"}`)

	if !strings.Contains(output, "Repos depending on") {
		t.Fatalf("expected dependency results, got:\n%s", output)
	}
}

// TestFleet_MCPServer_ErrorCases verifies MCP handler error paths.
func TestFleet_MCPServer_ErrorCases(t *testing.T) {
	fg := loadTestFleetGraph(t)

	// Missing required param for fleet_search.
	output := mcpToolCall(t, fg, "fleet_search", `{}`)
	if !strings.Contains(output, "isError") {
		t.Fatalf("expected isError for missing query param, got:\n%s", output)
	}

	// Unknown tool.
	output = mcpToolCall(t, fg, "nonexistent_tool", `{}`)
	if !strings.Contains(output, "isError") {
		t.Fatalf("expected isError for unknown tool, got:\n%s", output)
	}

	// fleet_file_context missing path.
	output = mcpToolCall(t, fg, "fleet_file_context", `{"repo":"go-js"}`)
	if !strings.Contains(output, "isError") {
		t.Fatalf("expected isError for missing path, got:\n%s", output)
	}

	// fleet_symbol_lookup missing name.
	output = mcpToolCall(t, fg, "fleet_symbol_lookup", `{}`)
	if !strings.Contains(output, "isError") {
		t.Fatalf("expected isError for missing name, got:\n%s", output)
	}
}

// TestFleet_DuckDBQuery verifies fleet query with DuckDB CLI (skipped if not available).
func TestFleet_DuckDBQuery(t *testing.T) {
	if _, err := exec.LookPath("duckdb"); err != nil {
		t.Skipf("duckdb CLI not available: %v", err)
	}

	bin := falconBin(t)
	root := repoRootDir(t)
	repo1 := filepath.Join(root, "testdata", "tinyrepo_go_js")
	repo2 := filepath.Join(root, "testdata", "tinyrepo_go_py")

	runFalcon(t, bin, "fleet", "sync", "--repos", repo1+","+repo2)

	m := &fleet.Manifest{
		Version: "1",
		Repos: []fleet.RepoEntry{
			{Path: repo1, Name: "go-js"},
			{Path: repo2, Name: "go-py"},
		},
	}

	// Basic query: count files per repo.
	result, err := fleet.RunQuery(m, "SELECT _repo, COUNT(*) as n FROM all_files GROUP BY _repo ORDER BY _repo", "table")
	if err != nil {
		t.Fatalf("RunQuery: %v", err)
	}
	if !strings.Contains(result, "go-js") || !strings.Contains(result, "go-py") {
		t.Fatalf("expected both repos in result, got:\n%s", result)
	}

	// JSON format.
	result, err = fleet.RunQuery(m, "SELECT DISTINCT _repo FROM all_files ORDER BY _repo", "json")
	if err != nil {
		t.Fatalf("RunQuery json: %v", err)
	}
	if !strings.Contains(result, "go-js") {
		t.Fatalf("expected go-js in json output, got:\n%s", result)
	}

	// CSV format.
	result, err = fleet.RunQuery(m, "SELECT DISTINCT _repo FROM all_files ORDER BY _repo", "csv")
	if err != nil {
		t.Fatalf("RunQuery csv: %v", err)
	}
	if !strings.Contains(result, "go-js") {
		t.Fatalf("expected go-js in csv output, got:\n%s", result)
	}
}

// TestFleet_SanitizeSQLIdent verifies SQL identifier sanitization.
func TestFleet_SanitizeSQLIdent(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"my-app", "my_app"},
		{"app_v2", "app_v2"},
		{"my.app.name", "my_app_name"},
		{"app/web", "app_web"},
		{"Simple", "Simple"},
		{"a b c", "a_b_c"},
	}
	for _, tc := range tests {
		got := fleet.SanitizeSQLIdent(tc.input)
		if got != tc.want {
			t.Errorf("SanitizeSQLIdent(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- helpers ---

// mcpToolCall sends a single tool call to the fleet MCP server and returns the raw output.
func mcpToolCall(t *testing.T, fg *fleet.FleetGraph, toolName, argsJSON string) string {
	t.Helper()
	srv := fleet.NewFleetServer(fg)

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rr, rw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(pr, rw)
	}()

	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, toolName, argsJSON) + "\n"
	if _, err := pw.WriteString(requests); err != nil {
		t.Fatal(err)
	}
	pw.Close()
	<-done

	buf := make([]byte, 128*1024)
	n, _ := rr.Read(buf)
	rr.Close()
	rw.Close()
	return string(buf[:n])
}

// loadTestFleetGraph indexes the two test fixtures and loads them as a FleetGraph.
func loadTestFleetGraph(t *testing.T) *fleet.FleetGraph {
	t.Helper()

	bin := falconBin(t)
	root := repoRootDir(t)
	repo1 := filepath.Join(root, "testdata", "tinyrepo_go_js")
	repo2 := filepath.Join(root, "testdata", "tinyrepo_go_py")

	// Ensure both are indexed.
	runFalcon(t, bin, "fleet", "sync", "--repos", repo1+","+repo2)

	ctx := context.Background()
	m := &fleet.Manifest{
		Version: "1",
		Repos: []fleet.RepoEntry{
			{Path: repo1, Name: "go-js"},
			{Path: repo2, Name: "go-py"},
		},
	}
	fg, err := fleet.LoadFleetGraph(ctx, m)
	if err != nil {
		t.Fatalf("LoadFleetGraph: %v", err)
	}
	return fg
}

// writeFleetManifest writes a fleet.json manifest file.
func writeFleetManifest(t *testing.T, path string, repos []fleet.RepoEntry) {
	t.Helper()
	m := fleet.Manifest{
		Version: "1",
		Repos:   repos,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// Ensure mcp import is used (needed for FleetServer type).
var _ = mcp.LoadGraph
