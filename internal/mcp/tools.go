package mcp

// ToolDef describes an MCP tool.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// AllTools returns the list of tools exposed by this MCP server.
func AllTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "falcon_architecture",
			Description: "Get a high-level architecture overview of the repository: languages, packages, dependency counts, and internal package listing.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "falcon_file_context",
			Description: "Get detailed context for a specific file: symbols defined in it, what it imports, and what other files depend on it.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Repo-relative file path (e.g. internal/extract/go.go)",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "falcon_symbol_lookup",
			Description: "Look up a symbol by name and see its location, relationships (callers, callees, references).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Symbol name to search for (case-insensitive)",
					},
					"kind": map[string]any{
						"type":        "string",
						"description": "Optional: filter by symbol kind (function, method, type, variable, const)",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "falcon_package_info",
			Description: "Get information about a package: its files, symbols, dependencies, and dependents.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Package name (e.g. internal/extract or github.com/spf13/cobra)",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "falcon_search",
			Description: "Search for files, symbols, or packages by name (substring match).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query (substring, case-insensitive)",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "Scope to search: file, symbol, package, or empty for all",
						"enum":        []string{"", "file", "symbol", "package"},
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "falcon_refresh",
			Description: "Re-index the repository and reload the code knowledge graph. Call this after major refactoring (renamed packages, moved files, changed dependency structure). Not needed for small edits.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}
