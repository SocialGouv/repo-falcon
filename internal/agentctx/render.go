package agentctx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"repofalcon/internal/artifacts"
)

// BuildSummary loads Parquet artifacts from snapshotDir and produces an ArchSummary.
func BuildSummary(ctx context.Context, snapshotDir string) (*ArchSummary, error) {
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

	// Index files by ID.
	fileByID := make(map[string]artifacts.FileRow, len(files))
	for _, f := range files {
		fileByID[f.FileID] = f
	}

	// Index packages by ID.
	pkgByID := make(map[string]artifacts.PackageRow, len(packages))
	for _, p := range packages {
		pkgByID[p.PackageID] = p
	}

	// Language stats.
	langStats := map[string]int{}
	for _, f := range files {
		if f.Language != "" && f.Language != "unknown" {
			langStats[f.Language]++
		}
	}

	// Symbol stats.
	symbolKinds := map[string]int{}
	// Map file_id → symbols.
	symsByFile := map[string][]artifacts.SymbolRow{}
	for _, s := range symbols {
		symbolKinds[s.Kind]++
		symsByFile[s.FileID] = append(symsByFile[s.FileID], s)
	}

	// Build edge indexes.
	// fileImports: file_id → []package_id (IMPORTS edges)
	fileImports := map[string][]string{}
	// pkgFiles: package_id → []file_id (CONTAINS edges)
	pkgFiles := map[string][]string{}

	for _, e := range edges {
		switch e.EdgeType {
		case "IMPORTS":
			fileImports[e.SrcID] = append(fileImports[e.SrcID], e.DstID)
		case "CONTAINS":
			pkgFiles[e.SrcID] = append(pkgFiles[e.SrcID], e.DstID)
		}
	}

	// Build internal package summaries and dependency graph.
	var internalPkgs []PackageSummary
	depGraph := map[string][]string{}        // internal pkg → imported pkgs (internal only)
	reverseDepGraph := map[string][]string{} // internal pkg → importing pkgs (internal only)
	extUsedBy := map[string][]string{}       // ext pkg name → internal pkg names that use it

	internalPkgIDs := map[string]bool{}
	for _, p := range packages {
		if p.IsInternal {
			internalPkgIDs[p.PackageID] = true
		}
	}

	for _, p := range packages {
		if !p.IsInternal {
			continue
		}

		// Files in this package.
		fileIDs := pkgFiles[p.PackageID]
		var fileNames []string
		for _, fid := range fileIDs {
			if f, ok := fileByID[fid]; ok {
				fileNames = append(fileNames, filepath.Base(f.Path))
			}
		}
		sort.Strings(fileNames)

		// Symbols in this package (via its files).
		var syms []SymbolBrief
		for _, fid := range fileIDs {
			for _, s := range symsByFile[fid] {
				path := ""
				if f, ok := fileByID[fid]; ok {
					path = f.Path
				}
				syms = append(syms, SymbolBrief{
					Name:          s.Name,
					QualifiedName: s.QualifiedName,
					Kind:          s.Kind,
					FilePath:      path,
					Line:          s.StartLine,
				})
			}
		}
		sort.Slice(syms, func(i, j int) bool {
			if syms[i].FilePath != syms[j].FilePath {
				return syms[i].FilePath < syms[j].FilePath
			}
			return syms[i].Line < syms[j].Line
		})

		// Imports: collect all packages imported by this package's files.
		importedSet := map[string]bool{}
		for _, fid := range fileIDs {
			for _, impID := range fileImports[fid] {
				importedSet[impID] = true
			}
		}

		var imports []string
		for impID := range importedSet {
			if imp, ok := pkgByID[impID]; ok {
				imports = append(imports, imp.Name)
				if internalPkgIDs[impID] {
					depGraph[p.Name] = appendUnique(depGraph[p.Name], imp.Name)
					reverseDepGraph[imp.Name] = appendUnique(reverseDepGraph[imp.Name], p.Name)
				} else {
					extUsedBy[imp.Name] = appendUnique(extUsedBy[imp.Name], p.Name)
				}
			}
		}
		sort.Strings(imports)

		internalPkgs = append(internalPkgs, PackageSummary{
			Name:      p.Name,
			Ecosystem: p.Ecosystem,
			Files:     fileNames,
			Symbols:   syms,
			Imports:   imports,
		})
	}

	// Sort internal packages by name.
	sort.Slice(internalPkgs, func(i, j int) bool {
		return internalPkgs[i].Name < internalPkgs[j].Name
	})

	// Fill ImportedBy from reverseDepGraph.
	for i := range internalPkgs {
		internalPkgs[i].ImportedBy = reverseDepGraph[internalPkgs[i].Name]
		sort.Strings(internalPkgs[i].ImportedBy)
	}

	// Sort dep graph values.
	for k := range depGraph {
		sort.Strings(depGraph[k])
	}
	for k := range reverseDepGraph {
		sort.Strings(reverseDepGraph[k])
	}

	// External deps.
	var extDeps []ExternalDep
	for _, p := range packages {
		if p.IsInternal {
			continue
		}
		usedBy := extUsedBy[p.Name]
		sort.Strings(usedBy)
		extDeps = append(extDeps, ExternalDep{
			Name:      p.Name,
			Ecosystem: p.Ecosystem,
			UsedBy:    usedBy,
		})
	}
	sort.Slice(extDeps, func(i, j int) bool {
		return extDeps[i].Name < extDeps[j].Name
	})

	return &ArchSummary{
		LangStats:       langStats,
		TotalFiles:      len(files),
		SymbolKinds:     symbolKinds,
		TotalSymbols:    len(symbols),
		InternalPkgs:    internalPkgs,
		ExternalDeps:    extDeps,
		DepGraph:        depGraph,
		ReverseDepGraph: reverseDepGraph,
	}, nil
}

// RenderMarkdown renders the summary as a markdown document for coding agents.
func RenderMarkdown(summary *ArchSummary) string {
	var b strings.Builder
	b.Grow(16 * 1024)

	f := func(format string, args ...any) { b.WriteString(fmt.Sprintf(format, args...)) }

	f("# Code Knowledge Graph\n\n")
	f("This file is auto-generated by `falcon agent-context`. It provides a structured\n")
	f("overview of the repository's code graph for use by coding agents.\n\n")

	// Overview.
	f("## Overview\n\n")
	f("- Total files: %d\n", summary.TotalFiles)

	// Languages sorted by count descending.
	type langCount struct {
		lang  string
		count int
	}
	var langs []langCount
	for l, c := range summary.LangStats {
		langs = append(langs, langCount{l, c})
	}
	sort.Slice(langs, func(i, j int) bool {
		if langs[i].count != langs[j].count {
			return langs[i].count > langs[j].count
		}
		return langs[i].lang < langs[j].lang
	})
	if len(langs) > 0 {
		parts := make([]string, len(langs))
		for i, lc := range langs {
			parts[i] = fmt.Sprintf("%s (%d)", lc.lang, lc.count)
		}
		f("- Languages: %s\n", strings.Join(parts, ", "))
	}

	f("- Internal packages: %d\n", len(summary.InternalPkgs))
	f("- External dependencies: %d\n", len(summary.ExternalDeps))
	f("- Total symbols: %d", summary.TotalSymbols)
	if len(summary.SymbolKinds) > 0 {
		type kindCount struct {
			kind  string
			count int
		}
		var kinds []kindCount
		for k, c := range summary.SymbolKinds {
			kinds = append(kinds, kindCount{k, c})
		}
		sort.Slice(kinds, func(i, j int) bool {
			if kinds[i].count != kinds[j].count {
				return kinds[i].count > kinds[j].count
			}
			return kinds[i].kind < kinds[j].kind
		})
		parts := make([]string, len(kinds))
		for i, kc := range kinds {
			parts[i] = fmt.Sprintf("%s: %d", kc.kind, kc.count)
		}
		f(" (%s)", strings.Join(parts, ", "))
	}
	f("\n\n")

	// Package map.
	if len(summary.InternalPkgs) > 0 {
		f("## Package Map\n\n")
		for _, pkg := range summary.InternalPkgs {
			f("### %s\n\n", pkg.Name)
			if len(pkg.Files) > 0 {
				f("- **Files**: %s\n", strings.Join(pkg.Files, ", "))
			}
			if len(pkg.Symbols) > 0 {
				var parts []string
				for _, s := range pkg.Symbols {
					parts = append(parts, fmt.Sprintf("`%s` (%s)", s.Name, s.Kind))
				}
				// Show at most 20 symbols to keep context size reasonable.
				if len(parts) > 20 {
					parts = append(parts[:20], fmt.Sprintf("... and %d more", len(parts)-20))
				}
				f("- **Symbols**: %s\n", strings.Join(parts, ", "))
			}
			if len(pkg.Imports) > 0 {
				f("- **Imports**: %s\n", strings.Join(pkg.Imports, ", "))
			}
			if len(pkg.ImportedBy) > 0 {
				f("- **Imported by**: %s\n", strings.Join(pkg.ImportedBy, ", "))
			}
			f("\n")
		}
	}

	// Dependency graph (internal only).
	if len(summary.DepGraph) > 0 {
		f("## Internal Dependency Graph\n\n")
		f("```\n")
		keys := make([]string, 0, len(summary.DepGraph))
		for k := range summary.DepGraph {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, from := range keys {
			tos := summary.DepGraph[from]
			f("%s -> %s\n", from, strings.Join(tos, ", "))
		}
		f("```\n\n")
	}

	// External dependencies.
	if len(summary.ExternalDeps) > 0 {
		f("## External Dependencies\n\n")
		for _, dep := range summary.ExternalDeps {
			if len(dep.UsedBy) > 0 {
				f("- `%s` [%s] (used by: %s)\n", dep.Name, dep.Ecosystem, strings.Join(dep.UsedBy, ", "))
			} else {
				f("- `%s` [%s]\n", dep.Name, dep.Ecosystem)
			}
		}
		f("\n")
	}

	return b.String()
}

// RenderJSON renders the summary as indented JSON.
func RenderJSON(summary *ArchSummary) (string, error) {
	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WriteContext builds the summary and writes it to outPath.
func WriteContext(ctx context.Context, snapshotDir, outPath, format string) error {
	summary, err := BuildSummary(ctx, snapshotDir)
	if err != nil {
		return err
	}

	var content string
	switch format {
	case "json":
		content, err = RenderJSON(summary)
		if err != nil {
			return err
		}
	default:
		content = RenderMarkdown(summary)
	}

	dir := filepath.Dir(outPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	return os.WriteFile(outPath, []byte(content), 0o644)
}

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
