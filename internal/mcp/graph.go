package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"repofalcon/internal/artifacts"
)

// GraphIndex is an in-memory index of the code knowledge graph,
// built from Parquet artifacts at startup.
type GraphIndex struct {
	Files    []artifacts.FileRow
	Packages []artifacts.PackageRow
	Symbols  []artifacts.SymbolRow
	Edges    []artifacts.EdgeRow
	Findings []artifacts.FindingRow

	// Lookup maps.
	FileByID   map[string]artifacts.FileRow
	FileByPath map[string]artifacts.FileRow
	PkgByID    map[string]artifacts.PackageRow
	PkgByName  map[string]artifacts.PackageRow
	SymByID    map[string]artifacts.SymbolRow
	SymsByFile map[string][]artifacts.SymbolRow
	SymsByName map[string][]artifacts.SymbolRow

	// Edge indexes.
	EdgesBySrc  map[string][]artifacts.EdgeRow
	EdgesByDst  map[string][]artifacts.EdgeRow
	PkgFiles    map[string][]string // package_id → []file_id (CONTAINS)
	FileImports map[string][]string // file_id → []package_id (IMPORTS)

	// Workspace indexes.
	PkgsByScope map[string][]artifacts.PackageRow // scope (workspace member name) → packages
}

// LoadGraph loads all Parquet artifacts from snapshotDir and builds the index.
func LoadGraph(ctx context.Context, snapshotDir string) (*GraphIndex, error) {
	files, err := artifacts.ReadFilesParquet(ctx, filepath.Join(snapshotDir, "files.parquet"))
	if err != nil {
		return nil, fmt.Errorf("read files: %w", err)
	}
	packages, err := artifacts.ReadPackagesParquet(ctx, filepath.Join(snapshotDir, "packages.parquet"))
	if err != nil {
		return nil, fmt.Errorf("read packages: %w", err)
	}
	symbols, err := artifacts.ReadSymbolsParquet(ctx, filepath.Join(snapshotDir, "symbols.parquet"))
	if err != nil {
		return nil, fmt.Errorf("read symbols: %w", err)
	}
	edges, err := artifacts.ReadEdgesParquet(ctx, filepath.Join(snapshotDir, "edges.parquet"))
	if err != nil {
		return nil, fmt.Errorf("read edges: %w", err)
	}
	findings, err := artifacts.ReadFindingsParquet(ctx, filepath.Join(snapshotDir, "findings.parquet"))
	if err != nil {
		return nil, fmt.Errorf("read findings: %w", err)
	}

	g := &GraphIndex{
		Files:       files,
		Packages:    packages,
		Symbols:     symbols,
		Edges:       edges,
		Findings:    findings,
		FileByID:    make(map[string]artifacts.FileRow, len(files)),
		FileByPath:  make(map[string]artifacts.FileRow, len(files)),
		PkgByID:     make(map[string]artifacts.PackageRow, len(packages)),
		PkgByName:   make(map[string]artifacts.PackageRow, len(packages)),
		SymByID:     make(map[string]artifacts.SymbolRow, len(symbols)),
		SymsByFile:  make(map[string][]artifacts.SymbolRow),
		SymsByName:  make(map[string][]artifacts.SymbolRow),
		EdgesBySrc:  make(map[string][]artifacts.EdgeRow),
		EdgesByDst:  make(map[string][]artifacts.EdgeRow),
		PkgFiles:    make(map[string][]string),
		FileImports: make(map[string][]string),
		PkgsByScope: make(map[string][]artifacts.PackageRow),
	}

	for _, f := range files {
		g.FileByID[f.FileID] = f
		g.FileByPath[f.Path] = f
	}
	for _, p := range packages {
		g.PkgByID[p.PackageID] = p
		g.PkgByName[p.Name] = p
		if p.Scope != "" {
			g.PkgsByScope[p.Scope] = append(g.PkgsByScope[p.Scope], p)
		}
	}
	for _, s := range symbols {
		g.SymByID[s.SymbolID] = s
		g.SymsByFile[s.FileID] = append(g.SymsByFile[s.FileID], s)
		nameLower := strings.ToLower(s.Name)
		g.SymsByName[nameLower] = append(g.SymsByName[nameLower], s)
	}
	for _, e := range edges {
		g.EdgesBySrc[e.SrcID] = append(g.EdgesBySrc[e.SrcID], e)
		g.EdgesByDst[e.DstID] = append(g.EdgesByDst[e.DstID], e)
		switch e.EdgeType {
		case "CONTAINS":
			g.PkgFiles[e.SrcID] = append(g.PkgFiles[e.SrcID], e.DstID)
		case "IMPORTS":
			g.FileImports[e.SrcID] = append(g.FileImports[e.SrcID], e.DstID)
		}
	}

	return g, nil
}

// FileContext returns a text summary of a file: its symbols, imports, and dependents.
func (g *GraphIndex) FileContext(path string) string {
	f, ok := g.FileByPath[path]
	if !ok {
		return fmt.Sprintf("File not found: %s", path)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", f.Path)
	fmt.Fprintf(&b, "- Language: %s\n", f.Language)
	if f.Lines != nil {
		fmt.Fprintf(&b, "- Lines: %d\n", *f.Lines)
	}
	fmt.Fprintf(&b, "- Size: %d bytes\n\n", f.SizeBytes)

	// Symbols defined in this file.
	syms := g.SymsByFile[f.FileID]
	if len(syms) > 0 {
		b.WriteString("## Symbols\n\n")
		for _, s := range syms {
			fmt.Fprintf(&b, "- `%s` (%s) line %d\n", s.QualifiedName, s.Kind, s.StartLine)
		}
		b.WriteString("\n")
	}

	// Imports.
	impIDs := g.FileImports[f.FileID]
	if len(impIDs) > 0 {
		b.WriteString("## Imports\n\n")
		var names []string
		for _, id := range impIDs {
			if p, ok := g.PkgByID[id]; ok {
				names = append(names, p.Name)
			}
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(&b, "- %s\n", n)
		}
		b.WriteString("\n")
	}

	// Reverse: files that import the same package as this file belongs to.
	// Find which package contains this file.
	var containingPkgID string
	for _, e := range g.EdgesByDst[f.FileID] {
		if e.EdgeType == "CONTAINS" {
			containingPkgID = e.SrcID
			break
		}
	}
	if containingPkgID != "" {
		var dependents []string
		for _, e := range g.EdgesByDst[containingPkgID] {
			if e.EdgeType == "IMPORTS" {
				if srcFile, ok := g.FileByID[e.SrcID]; ok {
					dependents = append(dependents, srcFile.Path)
				}
			}
		}
		if len(dependents) > 0 {
			sort.Strings(dependents)
			b.WriteString("## Depended on by\n\n")
			for _, d := range dependents {
				fmt.Fprintf(&b, "- %s\n", d)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// SymbolLookup returns info about symbols matching the given name.
func (g *GraphIndex) SymbolLookup(name string, kind string) string {
	nameLower := strings.ToLower(name)
	syms := g.SymsByName[nameLower]
	if len(syms) == 0 {
		return fmt.Sprintf("No symbols found matching: %s", name)
	}

	if kind != "" {
		var filtered []artifacts.SymbolRow
		for _, s := range syms {
			if strings.EqualFold(s.Kind, kind) {
				filtered = append(filtered, s)
			}
		}
		syms = filtered
		if len(syms) == 0 {
			return fmt.Sprintf("No %s symbols found matching: %s", kind, name)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Symbol: %s\n\n", name)
	fmt.Fprintf(&b, "Found %d match(es):\n\n", len(syms))

	for _, s := range syms {
		path := ""
		if f, ok := g.FileByID[s.FileID]; ok {
			path = f.Path
		}
		fmt.Fprintf(&b, "## `%s` (%s)\n\n", s.QualifiedName, s.Kind)
		fmt.Fprintf(&b, "- File: %s:%d\n", path, s.StartLine)
		fmt.Fprintf(&b, "- Language: %s\n", s.Language)
		if s.Signature != nil {
			fmt.Fprintf(&b, "- Signature: `%s`\n", *s.Signature)
		}

		// Edges from this symbol.
		outEdges := g.EdgesBySrc[s.SymbolID]
		inEdges := g.EdgesByDst[s.SymbolID]

		var calls, refs, calledBy, referencedBy []string
		for _, e := range outEdges {
			switch e.EdgeType {
			case "CALLS":
				if t, ok := g.SymByID[e.DstID]; ok {
					calls = append(calls, t.QualifiedName)
				}
			case "REFERENCES":
				if t, ok := g.SymByID[e.DstID]; ok {
					refs = append(refs, t.QualifiedName)
				}
			}
		}
		for _, e := range inEdges {
			switch e.EdgeType {
			case "CALLS":
				if t, ok := g.SymByID[e.SrcID]; ok {
					calledBy = append(calledBy, t.QualifiedName)
				}
			case "REFERENCES":
				if t, ok := g.SymByID[e.SrcID]; ok {
					referencedBy = append(referencedBy, t.QualifiedName)
				}
			}
		}

		if len(calls) > 0 {
			sort.Strings(calls)
			fmt.Fprintf(&b, "- Calls: %s\n", strings.Join(calls, ", "))
		}
		if len(calledBy) > 0 {
			sort.Strings(calledBy)
			fmt.Fprintf(&b, "- Called by: %s\n", strings.Join(calledBy, ", "))
		}
		if len(refs) > 0 {
			sort.Strings(refs)
			fmt.Fprintf(&b, "- References: %s\n", strings.Join(refs, ", "))
		}
		if len(referencedBy) > 0 {
			sort.Strings(referencedBy)
			fmt.Fprintf(&b, "- Referenced by: %s\n", strings.Join(referencedBy, ", "))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// PackageInfo returns a summary for a package by name.
func (g *GraphIndex) PackageInfo(name string) string {
	pkg, ok := g.PkgByName[name]
	if !ok {
		return fmt.Sprintf("Package not found: %s", name)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Package: %s\n\n", pkg.Name)
	fmt.Fprintf(&b, "- Ecosystem: %s\n", pkg.Ecosystem)
	fmt.Fprintf(&b, "- Internal: %v\n", pkg.IsInternal)
	if pkg.Scope != "" {
		fmt.Fprintf(&b, "- Workspace member: %s\n", pkg.Scope)
	}
	if pkg.RootPath != nil {
		fmt.Fprintf(&b, "- Root path: %s\n", *pkg.RootPath)
	}
	if pkg.Version != "" {
		fmt.Fprintf(&b, "- Version: %s\n", pkg.Version)
	}
	b.WriteString("\n")

	// Files.
	fileIDs := g.PkgFiles[pkg.PackageID]
	if len(fileIDs) > 0 {
		b.WriteString("## Files\n\n")
		var paths []string
		for _, fid := range fileIDs {
			if f, ok := g.FileByID[fid]; ok {
				paths = append(paths, f.Path)
			}
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	}

	// Symbols.
	var allSyms []artifacts.SymbolRow
	for _, fid := range fileIDs {
		allSyms = append(allSyms, g.SymsByFile[fid]...)
	}
	if len(allSyms) > 0 {
		b.WriteString("## Symbols\n\n")
		for _, s := range allSyms {
			path := ""
			if f, ok := g.FileByID[s.FileID]; ok {
				path = f.Path
			}
			fmt.Fprintf(&b, "- `%s` (%s) in %s:%d\n", s.QualifiedName, s.Kind, path, s.StartLine)
		}
		b.WriteString("\n")
	}

	// Dependencies and dependents.
	var deps, depBy []string
	for _, e := range g.EdgesBySrc[pkg.PackageID] {
		if e.EdgeType == "DEPENDS_ON" {
			if p, ok := g.PkgByID[e.DstID]; ok {
				deps = append(deps, p.Name)
			}
		}
	}
	for _, e := range g.EdgesByDst[pkg.PackageID] {
		if e.EdgeType == "DEPENDS_ON" {
			if p, ok := g.PkgByID[e.SrcID]; ok {
				depBy = append(depBy, p.Name)
			}
		}
		if e.EdgeType == "IMPORTS" {
			if f, ok := g.FileByID[e.SrcID]; ok {
				depBy = append(depBy, f.Path)
			}
		}
	}

	if len(deps) > 0 {
		sort.Strings(deps)
		b.WriteString("## Depends on\n\n")
		for _, d := range deps {
			fmt.Fprintf(&b, "- %s\n", d)
		}
		b.WriteString("\n")
	}
	if len(depBy) > 0 {
		sort.Strings(depBy)
		depBy = dedupStrings(depBy)
		b.WriteString("## Depended on by\n\n")
		for _, d := range depBy {
			fmt.Fprintf(&b, "- %s\n", d)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Architecture returns a high-level overview of the graph.
func (g *GraphIndex) Architecture() string {
	var b strings.Builder

	langStats := map[string]int{}
	for _, f := range g.Files {
		if f.Language != "" && f.Language != "unknown" {
			langStats[f.Language]++
		}
	}
	symbolKinds := map[string]int{}
	for _, s := range g.Symbols {
		symbolKinds[s.Kind]++
	}

	fmt.Fprintf(&b, "# Architecture Overview\n\n")
	fmt.Fprintf(&b, "- Files: %d\n", len(g.Files))

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

	var intCount, extCount int
	for _, p := range g.Packages {
		if p.IsInternal {
			intCount++
		} else {
			extCount++
		}
	}
	fmt.Fprintf(&b, "- Internal packages: %d\n", intCount)
	fmt.Fprintf(&b, "- External dependencies: %d\n", extCount)
	if len(g.PkgsByScope) > 0 {
		fmt.Fprintf(&b, "- Workspace members: %d\n", len(g.PkgsByScope))
	}
	fmt.Fprintf(&b, "- Symbols: %d\n", len(g.Symbols))
	fmt.Fprintf(&b, "- Edges: %d\n\n", len(g.Edges))

	// Workspace members section (if any).
	if len(g.PkgsByScope) > 0 {
		b.WriteString("## Workspace Members\n\n")
		var scopes []string
		for scope := range g.PkgsByScope {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		for _, scope := range scopes {
			pkgs := g.PkgsByScope[scope]
			fmt.Fprintf(&b, "- **%s** (%d packages)\n", scope, len(pkgs))
		}
		b.WriteString("\n")
	}

	// Internal package list with their files.
	b.WriteString("## Internal Packages\n\n")
	var internal []artifacts.PackageRow
	for _, p := range g.Packages {
		if p.IsInternal {
			internal = append(internal, p)
		}
	}
	sort.Slice(internal, func(i, j int) bool { return internal[i].Name < internal[j].Name })
	for _, p := range internal {
		fileIDs := g.PkgFiles[p.PackageID]
		var paths []string
		for _, fid := range fileIDs {
			if f, ok := g.FileByID[fid]; ok {
				paths = append(paths, filepath.Base(f.Path))
			}
		}
		sort.Strings(paths)
		fmt.Fprintf(&b, "- **%s**: %s\n", p.Name, strings.Join(paths, ", "))
	}
	b.WriteString("\n")

	return b.String()
}

// Search finds files or symbols matching a query string (substring match).
func (g *GraphIndex) Search(query, scope string) string {
	queryLower := strings.ToLower(query)
	var b strings.Builder
	fmt.Fprintf(&b, "# Search: %s (scope: %s)\n\n", query, scope)

	found := 0
	const maxResults = 30

	if scope == "" || scope == "file" {
		b.WriteString("## Files\n\n")
		for _, f := range g.Files {
			if found >= maxResults {
				break
			}
			if strings.Contains(strings.ToLower(f.Path), queryLower) {
				fmt.Fprintf(&b, "- %s (%s, %d lines)\n", f.Path, f.Language, derefInt32(f.Lines))
				found++
			}
		}
		b.WriteString("\n")
	}

	if scope == "" || scope == "symbol" {
		found = 0
		b.WriteString("## Symbols\n\n")
		for _, s := range g.Symbols {
			if found >= maxResults {
				b.WriteString("... (truncated)\n")
				break
			}
			if strings.Contains(strings.ToLower(s.Name), queryLower) ||
				strings.Contains(strings.ToLower(s.QualifiedName), queryLower) {
				path := ""
				if f, ok := g.FileByID[s.FileID]; ok {
					path = f.Path
				}
				fmt.Fprintf(&b, "- `%s` (%s) in %s:%d\n", s.QualifiedName, s.Kind, path, s.StartLine)
				found++
			}
		}
		b.WriteString("\n")
	}

	if scope == "" || scope == "package" {
		found = 0
		b.WriteString("## Packages\n\n")
		for _, p := range g.Packages {
			if found >= maxResults {
				break
			}
			if strings.Contains(strings.ToLower(p.Name), queryLower) {
				label := "external"
				if p.IsInternal {
					label = "internal"
				}
				fmt.Fprintf(&b, "- %s [%s] (%s)\n", p.Name, p.Ecosystem, label)
				found++
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// WorkspaceInfo returns workspace/monorepo structure details.
// If member is non-empty, returns detail for that specific member.
func (g *GraphIndex) WorkspaceInfo(member string) string {
	if len(g.PkgsByScope) == 0 {
		return "No workspace/monorepo structure detected in this repository."
	}

	var b strings.Builder

	if member != "" {
		// Detail for a specific member.
		pkgs, ok := g.PkgsByScope[member]
		if !ok {
			return fmt.Sprintf("Workspace member not found: %s\nAvailable members: %s", member, g.workspaceMemberList())
		}

		fmt.Fprintf(&b, "# Workspace Member: %s\n\n", member)
		if len(pkgs) > 0 && pkgs[0].RootPath != nil {
			fmt.Fprintf(&b, "- Root: %s\n", *pkgs[0].RootPath)
		}
		if len(pkgs) > 0 && pkgs[0].ManifestPath != nil {
			fmt.Fprintf(&b, "- Manifest: %s\n", *pkgs[0].ManifestPath)
		}
		fmt.Fprintf(&b, "- Packages: %d\n\n", len(pkgs))

		b.WriteString("## Packages\n\n")
		for _, p := range pkgs {
			fileCount := len(g.PkgFiles[p.PackageID])
			fmt.Fprintf(&b, "- %s (%s, %d files)\n", p.Name, p.Ecosystem, fileCount)
		}
		b.WriteString("\n")

		// Cross-member dependencies.
		b.WriteString("## Cross-member Dependencies\n\n")
		depMembers := map[string]bool{}
		for _, p := range pkgs {
			for _, fid := range g.PkgFiles[p.PackageID] {
				for _, impID := range g.FileImports[fid] {
					if impPkg, ok := g.PkgByID[impID]; ok && impPkg.Scope != "" && impPkg.Scope != member {
						depMembers[impPkg.Scope] = true
					}
				}
			}
		}
		if len(depMembers) > 0 {
			var deps []string
			for d := range depMembers {
				deps = append(deps, d)
			}
			sort.Strings(deps)
			for _, d := range deps {
				fmt.Fprintf(&b, "- → %s\n", d)
			}
		} else {
			b.WriteString("(none)\n")
		}
		b.WriteString("\n")

		return b.String()
	}

	// Overview of all workspace members.
	fmt.Fprintf(&b, "# Workspace Overview\n\n")
	fmt.Fprintf(&b, "- Members: %d\n\n", len(g.PkgsByScope))

	var scopes []string
	for scope := range g.PkgsByScope {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	for _, scope := range scopes {
		pkgs := g.PkgsByScope[scope]
		var rootPath string
		if len(pkgs) > 0 && pkgs[0].RootPath != nil {
			rootPath = *pkgs[0].RootPath
		}

		// Count files across all packages in this member.
		fileCount := 0
		for _, p := range pkgs {
			fileCount += len(g.PkgFiles[p.PackageID])
		}

		fmt.Fprintf(&b, "## %s\n\n", scope)
		if rootPath != "" {
			fmt.Fprintf(&b, "- Root: %s\n", rootPath)
		}
		fmt.Fprintf(&b, "- Packages: %d, Files: %d\n\n", len(pkgs), fileCount)
	}

	return b.String()
}

func (g *GraphIndex) workspaceMemberList() string {
	var scopes []string
	for scope := range g.PkgsByScope {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return strings.Join(scopes, ", ")
}

func dedupStrings(s []string) []string {
	if len(s) <= 1 {
		return s
	}
	out := make([]string, 0, len(s))
	prev := ""
	for _, v := range s {
		if v != prev {
			out = append(out, v)
			prev = v
		}
	}
	return out
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
