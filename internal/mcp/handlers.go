package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// HandleToolCall dispatches an MCP tool call to the appropriate handler.
func HandleToolCall(name string, args map[string]any, s *Server) (string, error) {
	switch name {
	case "falcon_architecture":
		return s.graph.Architecture(), nil

	case "falcon_file_context":
		path, _ := args["path"].(string)
		if path == "" {
			return "", fmt.Errorf("missing required parameter: path")
		}
		return s.graph.FileContext(path), nil

	case "falcon_symbol_lookup":
		symName, _ := args["name"].(string)
		if symName == "" {
			return "", fmt.Errorf("missing required parameter: name")
		}
		kind, _ := args["kind"].(string)
		return s.graph.SymbolLookup(symName, kind), nil

	case "falcon_package_info":
		pkgName, _ := args["name"].(string)
		if pkgName == "" {
			return "", fmt.Errorf("missing required parameter: name")
		}
		return s.graph.PackageInfo(pkgName), nil

	case "falcon_search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("missing required parameter: query")
		}
		scope, _ := args["scope"].(string)
		return s.graph.Search(query, scope), nil

	case "falcon_workspace_info":
		member, _ := args["member"].(string)
		return s.graph.WorkspaceInfo(member), nil

	case "falcon_refresh":
		return handleRefresh(s)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// handleRefresh re-runs index + snapshot on the repo, then reloads the graph.
func handleRefresh(s *Server) (string, error) {
	if s.RepoRoot == "" || s.SnapshotDir == "" {
		return "", fmt.Errorf("refresh not available: repo root or snapshot dir not configured")
	}

	// Find the falcon binary (ourselves).
	self, err := exec.LookPath("falcon")
	if err != nil {
		// Fallback: try to find it relative to the repo root or use os.Executable.
		selfExe, exeErr := selfExecutable()
		if exeErr != nil {
			return "", fmt.Errorf("cannot find falcon binary: %w", err)
		}
		self = selfExe
	}

	var log strings.Builder

	// Step 1: re-index.
	indexCmd := exec.Command(self, "index", "--repo", s.RepoRoot, "--out", s.SnapshotDir)
	indexOut, err := indexCmd.CombinedOutput()
	log.WriteString(string(indexOut))
	if err != nil {
		return "", fmt.Errorf("index failed: %w\n%s", err, indexOut)
	}

	// Step 2: snapshot.
	snapCmd := exec.Command(self, "snapshot", "--in", s.SnapshotDir, "--out", s.SnapshotDir)
	snapOut, err := snapCmd.CombinedOutput()
	log.WriteString(string(snapOut))
	if err != nil {
		return "", fmt.Errorf("snapshot failed: %w\n%s", err, snapOut)
	}

	// Step 3: reload graph in-memory.
	ctx := context.Background()
	g, err := LoadGraph(ctx, s.SnapshotDir)
	if err != nil {
		return "", fmt.Errorf("reload graph failed: %w", err)
	}
	s.ReloadGraph(g)

	return fmt.Sprintf("Graph refreshed successfully.\n- Files: %d\n- Packages: %d\n- Symbols: %d\n- Edges: %d",
		len(g.Files), len(g.Packages), len(g.Symbols), len(g.Edges)), nil
}
