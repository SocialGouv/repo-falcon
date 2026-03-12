package fleet

import (
	"fmt"
	"io"

	"repofalcon/internal/mcp"
)

// FleetServer wraps mcp.Server with fleet-aware cross-repo tools.
type FleetServer struct {
	srv   *mcp.Server
	fleet *FleetGraph
}

// NewFleetServer creates an MCP server backed by the fleet graph.
func NewFleetServer(fg *FleetGraph) *FleetServer {
	fs := &FleetServer{fleet: fg}
	fs.srv = mcp.NewCustomServer(fleetTools(), fs.handleToolCall)
	return fs
}

// Serve starts the MCP server over stdio.
func (fs *FleetServer) Serve(r io.Reader, w io.Writer) error {
	return fs.srv.Serve(r, w)
}

func (fs *FleetServer) handleToolCall(name string, args map[string]any, _ *mcp.Server) (string, error) {
	switch name {
	case "fleet_overview":
		return fs.fleet.FleetOverview(), nil

	case "fleet_search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("missing required parameter: query")
		}
		scope, _ := args["scope"].(string)
		repo, _ := args["repo"].(string)
		if repo != "" {
			rg, ok := fs.fleet.ByName[repo]
			if !ok {
				return "", fmt.Errorf("repo not found: %s (available: %s)", repo, fs.fleet.repoNames())
			}
			return rg.Graph.Search(query, scope), nil
		}
		return fs.fleet.SearchAll(query, scope), nil

	case "fleet_repo_architecture":
		repo, _ := args["repo"].(string)
		if repo == "" {
			return "", fmt.Errorf("missing required parameter: repo")
		}
		return fs.fleet.RepoArchitecture(repo)

	case "fleet_file_context":
		repo, _ := args["repo"].(string)
		path, _ := args["path"].(string)
		if repo == "" || path == "" {
			return "", fmt.Errorf("missing required parameters: repo, path")
		}
		return fs.fleet.RepoFileContext(repo, path)

	case "fleet_symbol_lookup":
		symName, _ := args["name"].(string)
		if symName == "" {
			return "", fmt.Errorf("missing required parameter: name")
		}
		kind, _ := args["kind"].(string)
		repo, _ := args["repo"].(string)
		return fs.fleet.SymbolLookup(repo, symName, kind), nil

	case "fleet_find_repos_by_dependency":
		dep, _ := args["dependency"].(string)
		if dep == "" {
			return "", fmt.Errorf("missing required parameter: dependency")
		}
		return fs.fleet.FindReposByDependency(dep), nil

	case "fleet_common_dependencies":
		return fs.fleet.CommonDependencies(), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
