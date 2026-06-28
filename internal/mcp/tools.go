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
			Name:        "falcon_path",
			Description: "Find the shortest call/reference path between two symbols (how are A and B connected?).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from": map[string]any{
						"type":        "string",
						"description": "Source symbol name",
					},
					"to": map[string]any{
						"type":        "string",
						"description": "Target symbol name",
					},
				},
				"required": []string{"from", "to"},
			},
		},
		{
			Name:        "falcon_hubs",
			Description: "List the most connected symbols (degree centrality over calls/references) — the core abstractions / god nodes.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"top": map[string]any{
						"type":        "number",
						"description": "How many hubs to return (default 20)",
					},
				},
			},
		},
		{
			Name:        "falcon_communities",
			Description: "Cluster the symbol graph into communities of related symbols (deterministic label propagation, no LLM). Useful for a high-level map of cohesive areas.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"top": map[string]any{
						"type":        "number",
						"description": "How many of the largest communities to return (default 25)",
					},
				},
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
			Name:        "falcon_workspace_info",
			Description: "Get workspace/monorepo structure: workspace members, their packages, and cross-member dependencies. Returns empty if the repository is not a monorepo.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"member": map[string]any{
						"type":        "string",
						"description": "Optional: specific workspace member name to get details for. Omit for overview of all members.",
					},
				},
			},
		},
		{
			Name:        "falcon_insights",
			Description: "Surface non-obvious structure: surprising cross-cluster connections and suggested questions about the codebase.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"top": map[string]any{"type": "number", "description": "How many surprising connections to return (default 10)"},
				},
			},
		},
		{
			Name: "falcon_remember",
			Description: "Save the outcome of a graph query into work memory (useful/dead_end/corrected) " +
				"so `falcon reflect` can learn which sources pay off. Call after answering from the graph.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question":   map[string]any{"type": "string", "description": "The question that was asked"},
					"answer":     map[string]any{"type": "string", "description": "The answer given"},
					"type":       map[string]any{"type": "string", "description": "query | path | explain"},
					"nodes":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Symbol names cited in the answer"},
					"outcome":    map[string]any{"type": "string", "enum": []string{"useful", "dead_end", "corrected"}, "description": "Outcome signal"},
					"correction": map[string]any{"type": "string", "description": "The right answer, when outcome is corrected"},
				},
				"required": []string{"question", "outcome"},
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
