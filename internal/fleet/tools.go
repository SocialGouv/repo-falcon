package fleet

import "repofalcon/internal/mcp"

func fleetTools() []mcp.ToolDef {
	return []mcp.ToolDef{
		{
			Name:        "fleet_overview",
			Description: "Get a summary of all repositories in the fleet: repo names, file counts, languages, and symbol counts.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "fleet_search",
			Description: "Search for files, symbols, or packages across all fleet repos (or a specific repo). Use this to find implementations, patterns, or dependencies across your codebase.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query (substring, case-insensitive)",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "Scope: file, symbol, package, or empty for all",
						"enum":        []string{"", "file", "symbol", "package"},
					},
					"repo": map[string]any{
						"type":        "string",
						"description": "Optional: limit search to a specific repo by name",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "fleet_repo_architecture",
			Description: "Get architecture overview for a specific repo in the fleet: languages, packages, symbols, and dependency structure.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo": map[string]any{
						"type":        "string",
						"description": "Repository name (as shown in fleet_overview)",
					},
				},
				"required": []string{"repo"},
			},
		},
		{
			Name:        "fleet_file_context",
			Description: "Get file context (symbols, imports, dependents) for a file in a specific repo.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo": map[string]any{
						"type":        "string",
						"description": "Repository name",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Repo-relative file path",
					},
				},
				"required": []string{"repo", "path"},
			},
		},
		{
			Name:        "fleet_symbol_lookup",
			Description: "Look up a symbol across all fleet repos (or a specific repo). Returns location, callers, callees, and references.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Symbol name (case-insensitive substring match)",
					},
					"kind": map[string]any{
						"type":        "string",
						"description": "Optional: filter by kind (function, method, type, class, variable, const)",
					},
					"repo": map[string]any{
						"type":        "string",
						"description": "Optional: limit to a specific repo",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "fleet_find_repos_by_dependency",
			Description: "Find which repos depend on a given package/library. Example: 'next' to find Next.js apps, 'express' to find Express servers.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"dependency": map[string]any{
						"type":        "string",
						"description": "Package/library name to search for (substring match)",
					},
				},
				"required": []string{"dependency"},
			},
		},
		{
			Name:        "fleet_common_dependencies",
			Description: "List external dependencies shared across multiple repos in the fleet. Useful for understanding common libraries and frameworks.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}
