package mcp

import "fmt"

// HandleToolCall dispatches an MCP tool call to the appropriate handler.
func HandleToolCall(name string, args map[string]any, g *GraphIndex) (string, error) {
	switch name {
	case "falcon_architecture":
		return g.Architecture(), nil

	case "falcon_file_context":
		path, _ := args["path"].(string)
		if path == "" {
			return "", fmt.Errorf("missing required parameter: path")
		}
		return g.FileContext(path), nil

	case "falcon_symbol_lookup":
		symName, _ := args["name"].(string)
		if symName == "" {
			return "", fmt.Errorf("missing required parameter: name")
		}
		kind, _ := args["kind"].(string)
		return g.SymbolLookup(symName, kind), nil

	case "falcon_package_info":
		pkgName, _ := args["name"].(string)
		if pkgName == "" {
			return "", fmt.Errorf("missing required parameter: name")
		}
		return g.PackageInfo(pkgName), nil

	case "falcon_search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("missing required parameter: query")
		}
		scope, _ := args["scope"].(string)
		return g.Search(query, scope), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
