package fleet

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"repofalcon/internal/mcp"
)

// RepoGraph pairs a loaded graph with its repo metadata.
type RepoGraph struct {
	Name         string
	RepoPath     string
	ArtifactsDir string
	Graph        *mcp.GraphIndex
}

// FleetGraph holds all loaded per-repo graphs.
type FleetGraph struct {
	Repos  []RepoGraph
	ByName map[string]*RepoGraph
}

// LoadFleetGraph loads GraphIndex instances for every repo in the manifest.
func LoadFleetGraph(ctx context.Context, m *Manifest) (*FleetGraph, error) {
	fg := &FleetGraph{
		ByName: make(map[string]*RepoGraph, len(m.Repos)),
	}
	for _, entry := range m.Repos {
		artDir := entry.EffectiveArtifactsDir()
		g, err := mcp.LoadGraph(ctx, artDir)
		if err != nil {
			return nil, fmt.Errorf("load graph for %s: %w", entry.EffectiveName(), err)
		}
		rg := RepoGraph{
			Name:         entry.EffectiveName(),
			RepoPath:     entry.Path,
			ArtifactsDir: artDir,
			Graph:        g,
		}
		fg.Repos = append(fg.Repos, rg)
		fg.ByName[rg.Name] = &fg.Repos[len(fg.Repos)-1]
	}
	return fg, nil
}

// FleetOverview returns a summary of all repos.
func (fg *FleetGraph) FleetOverview() string {
	var b strings.Builder
	b.WriteString("# Fleet Overview\n\n")
	fmt.Fprintf(&b, "Total repos: %d\n\n", len(fg.Repos))

	for _, rg := range fg.Repos {
		g := rg.Graph
		fmt.Fprintf(&b, "## %s\n\n", rg.Name)
		fmt.Fprintf(&b, "- Path: %s\n", rg.RepoPath)
		fmt.Fprintf(&b, "- Files: %d\n", len(g.Files))
		fmt.Fprintf(&b, "- Packages: %d\n", len(g.Packages))
		fmt.Fprintf(&b, "- Symbols: %d\n", len(g.Symbols))

		langStats := map[string]int{}
		for _, f := range g.Files {
			if f.Language != "" && f.Language != "unknown" {
				langStats[f.Language]++
			}
		}
		type lc struct {
			l string
			c int
		}
		var langs []lc
		for l, c := range langStats {
			langs = append(langs, lc{l, c})
		}
		sort.Slice(langs, func(i, j int) bool { return langs[i].c > langs[j].c })
		if len(langs) > 0 {
			parts := make([]string, len(langs))
			for i, l := range langs {
				parts[i] = fmt.Sprintf("%s (%d)", l.l, l.c)
			}
			fmt.Fprintf(&b, "- Languages: %s\n", strings.Join(parts, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SearchAll runs a substring search across all repos.
func (fg *FleetGraph) SearchAll(query, scope string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Fleet Search: %q\n\n", query)
	for _, rg := range fg.Repos {
		result := rg.Graph.Search(query, scope)
		if !isEmptySearchResult(result) {
			fmt.Fprintf(&b, "## %s\n\n%s\n", rg.Name, result)
		}
	}
	return b.String()
}

// FindReposByDependency finds repos that import/depend on a given package name.
func (fg *FleetGraph) FindReposByDependency(pkgName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Repos depending on: %s\n\n", pkgName)
	nameLower := strings.ToLower(pkgName)
	found := false
	for _, rg := range fg.Repos {
		var matches []string
		for _, pkg := range rg.Graph.Packages {
			if strings.Contains(strings.ToLower(pkg.Name), nameLower) && !pkg.IsInternal {
				matches = append(matches, fmt.Sprintf("`%s` (%s)", pkg.Name, pkg.Ecosystem))
			}
		}
		if len(matches) > 0 {
			found = true
			fmt.Fprintf(&b, "- **%s**: %s\n", rg.Name, strings.Join(matches, ", "))
		}
	}
	if !found {
		b.WriteString("No repos found.\n")
	}
	return b.String()
}

// CommonDependencies finds packages used across multiple repos.
func (fg *FleetGraph) CommonDependencies() string {
	pkgRepos := map[string][]string{}
	for _, rg := range fg.Repos {
		seen := map[string]bool{}
		for _, pkg := range rg.Graph.Packages {
			if !pkg.IsInternal && !seen[pkg.Name] {
				seen[pkg.Name] = true
				pkgRepos[pkg.Name] = append(pkgRepos[pkg.Name], rg.Name)
			}
		}
	}

	type entry struct {
		name  string
		repos []string
	}
	var shared []entry
	for name, repos := range pkgRepos {
		if len(repos) > 1 {
			shared = append(shared, entry{name, repos})
		}
	}
	sort.Slice(shared, func(i, j int) bool { return len(shared[i].repos) > len(shared[j].repos) })

	var b strings.Builder
	b.WriteString("# Common Dependencies\n\n")
	if len(shared) == 0 {
		b.WriteString("No shared external dependencies found.\n")
		return b.String()
	}
	for _, e := range shared {
		sort.Strings(e.repos)
		fmt.Fprintf(&b, "- **%s** (%d repos): %s\n", e.name, len(e.repos), strings.Join(e.repos, ", "))
	}
	return b.String()
}

// RepoArchitecture returns architecture info for a specific repo.
func (fg *FleetGraph) RepoArchitecture(repoName string) (string, error) {
	rg, ok := fg.ByName[repoName]
	if !ok {
		return "", fmt.Errorf("repo not found: %s (available: %s)", repoName, fg.repoNames())
	}
	return rg.Graph.Architecture(), nil
}

// RepoFileContext returns file context scoped to a specific repo.
func (fg *FleetGraph) RepoFileContext(repoName, path string) (string, error) {
	rg, ok := fg.ByName[repoName]
	if !ok {
		return "", fmt.Errorf("repo not found: %s (available: %s)", repoName, fg.repoNames())
	}
	return rg.Graph.FileContext(path), nil
}

// SymbolLookup looks up a symbol within a specific repo (or all repos if repo is empty).
func (fg *FleetGraph) SymbolLookup(repo, name, kind string) string {
	if repo != "" {
		rg, ok := fg.ByName[repo]
		if !ok {
			return fmt.Sprintf("Repo not found: %s (available: %s)", repo, fg.repoNames())
		}
		return rg.Graph.SymbolLookup(name, kind)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Symbol lookup: %s (all repos)\n\n", name)
	found := false
	for _, rg := range fg.Repos {
		result := rg.Graph.SymbolLookup(name, kind)
		if !strings.Contains(result, "No symbols found") {
			found = true
			fmt.Fprintf(&b, "## %s\n\n%s\n", rg.Name, result)
		}
	}
	if !found {
		return fmt.Sprintf("No symbols found matching: %s", name)
	}
	return b.String()
}

func (fg *FleetGraph) repoNames() string {
	names := make([]string, len(fg.Repos))
	for i, rg := range fg.Repos {
		names[i] = rg.Name
	}
	return strings.Join(names, ", ")
}

// isEmptySearchResult checks if a search result contains only headers and no actual matches.
func isEmptySearchResult(result string) bool {
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			return false
		}
	}
	return true
}
