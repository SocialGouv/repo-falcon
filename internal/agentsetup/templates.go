package agentsetup

// falconInstructions is the markdown block injected into agent config files.
// It tells the agent how and when to use the falcon MCP tools.
const falconInstructions = `## RepoFalcon Code Knowledge Graph

This repository has a code knowledge graph available via MCP tools (` + "`falcon_*`" + `).

**When to use:**
- Before modifying a file, call ` + "`falcon_file_context`" + ` to see its dependencies and dependents
- Before refactoring, call ` + "`falcon_architecture`" + ` for package boundaries and dependency direction
- To find all usages of a symbol, use ` + "`falcon_symbol_lookup`" + ` instead of grep
- To search structurally, use ` + "`falcon_search`" + ` for packages, files, or symbols by name
- After major refactoring (renamed packages, moved files), call ` + "`falcon_refresh`" + ` to re-index

**Static context:** See ` + "`.falcon/CONTEXT.md`" + ` for a full architecture summary.`

// mcpServerConfig returns a JSON-compatible map for the falcon MCP server entry.
func mcpServerConfig(falconBin, snapshotDir string) map[string]any {
	return map[string]any{
		"command": falconBin,
		"args":    []string{"mcp", "serve", "--snapshot", snapshotDir, "--repo", "."},
	}
}
