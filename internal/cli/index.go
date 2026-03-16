package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/artifacts"
	"repofalcon/internal/extract"
	"repofalcon/internal/graph"
	"repofalcon/internal/logging"
	"repofalcon/internal/repo"
	"repofalcon/internal/workspace"
)

func newIndexCmd() *cobra.Command {
	var repoRootFlag string
	var out string

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index a repository and write artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()

			outDir := out
			if outDir == "" {
				outDir = "artifacts"
			}
			outDir = filepath.Clean(outDir)
			if err := artifacts.EnsureDir(outDir); err != nil {
				return err
			}

			repoRoot := repoRootFlag
			if repoRoot == "" {
				repoRoot = "."
			}
			repoRoot = filepath.Clean(repoRoot)

			meta := artifacts.NewMinimalMetadata(repoRoot, outDir)
			meta.Kind = "index"
			if err := artifacts.WriteMetadataJSON(filepath.Join(outDir, "metadata.json"), meta); err != nil {
				return err
			}

			recs, err := repo.Scan(repoRoot, repo.DefaultScanOptions())
			if err != nil {
				return err
			}

			// Detect workspace/monorepo structure (runs before extraction).
			ws := workspace.Detect(repoRoot)
			if !ws.IsEmpty() {
				lg.Info("workspace detected", "members", len(ws.Members))
			}

			modulePath := readGoModulePath(filepath.Join(repoRoot, "go.mod"))

			var fileRows []artifacts.FileRow
			var symbolRows []artifacts.SymbolRow
			var edgeRows []artifacts.EdgeRow
			packageByID := map[string]artifacts.PackageRow{}

			// Pre-compute Python top-level package directories.
			// A directory containing __init__.py is a Python package.
			pythonPkgDirs := make(map[string]bool)
			for _, fr := range recs {
				if filepath.Base(fr.RepoRelPath) == "__init__.py" {
					dir := filepath.Dir(fr.RepoRelPath)
					topLevel := strings.SplitN(dir, "/", 2)[0]
					pythonPkgDirs[topLevel] = true
					// Handle src/ layout: src/myapp/__init__.py -> "myapp".
					if strings.HasPrefix(dir, "src/") {
						rest := strings.TrimPrefix(dir, "src/")
						pythonPkgDirs[strings.SplitN(rest, "/", 2)[0]] = true
					}
				}
			}

			// Pre-parse Java files to collect package declarations (two-pass).
			javaExtracts := make(map[string]extract.JavaFile)
			javaRepoPkgs := make(map[string]bool)
			for _, fr := range recs {
				if fr.Language != "java" {
					continue
				}
				jf, err := extract.ExtractJavaFile(fr.RepoRelPath, fr.Content)
				if err != nil {
					lg.Warn("extract java pre-pass", "file", fr.RepoRelPath, "err", err)
					continue
				}
				javaExtracts[fr.RepoRelPath] = jf
				if jf.PackageName != "" {
					javaRepoPkgs[jf.PackageName] = true
				}
			}

			for _, fr := range recs {
				fileID := graph.NewFileID(fr.RepoRelPath)
				lines := fr.Lines
				fileRows = append(fileRows, artifacts.FileRow{
					FileID:        fileID,
					Path:          fr.RepoRelPath,
					Language:      fr.Language,
					Extension:     fr.Extension,
					SizeBytes:     fr.SizeBytes,
					ContentSHA256: fr.ContentSHA256,
					Lines:         &lines,
					IsGenerated:   false,
					IsTest:        isTestPath(fr.RepoRelPath),
				})

				switch fr.Language {
				case "go":
					gof, err := extract.ExtractGoFile(fr.RepoRelPath, fr.Content)
					if err != nil {
						return fmt.Errorf("extract go %s: %w", fr.RepoRelPath, err)
					}
					pkgID := graph.NewPackageID("go", gof.PackageName)
					packageByID[pkgID] = wsPackageRowFor("go", gof.PackageName, true, ws, fr.RepoRelPath)

					edgeRows = append(edgeRows, edgeRow(graph.EdgeContains, pkgID, string(graph.NodeTypePackage), fileID, string(graph.NodeTypeFile)))

					for _, imp := range gof.Imports {
						impID := graph.NewPackageID("go", imp)
						isInt := (strings.HasPrefix(imp, modulePath) && modulePath != "") || ws.IsGoWorkspaceImport(imp)
						packageByID[impID] = wsPackageRowFor("go", imp, isInt, ws, "")
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), impID, string(graph.NodeTypePackage)))
					}

					for _, s := range gof.Symbols {
						q := s.QualifiedName
						semKey := graph.SymbolKey("go", gof.PackageName, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						symID := graph.NewSymbolID("go", gof.PackageName, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						pkgIDPtr := pkgID
						symbolRows = append(symbolRows, artifacts.SymbolRow{
							SymbolID:          symID,
							FileID:            fileID,
							PackageID:         &pkgIDPtr,
							Language:          "go",
							Kind:              s.Kind,
							Name:              s.Name,
							QualifiedName:     q,
							Signature:         nil,
							SemanticKey:       semKey,
							StartLine:         int32(s.StartLine),
							StartCol:          int32(s.StartCol),
							EndLine:           int32(s.EndLine),
							EndCol:            int32(s.EndCol),
							Visibility:        nil,
							Modifiers:         nil,
							ContainerSymbolID: nil,
						})

						edgeRows = append(edgeRows,
							edgeRow(graph.EdgeDefines, fileID, string(graph.NodeTypeFile), symID, string(graph.NodeTypeSymbol)),
							edgeRow(graph.EdgeInFile, symID, string(graph.NodeTypeSymbol), fileID, string(graph.NodeTypeFile)),
						)
					}
				case "js", "ts":
					jsf, err := extract.ExtractJSFile(fr.RepoRelPath, fr.Content, fr.Language)
					if err != nil {
						return fmt.Errorf("extract js %s: %w", fr.RepoRelPath, err)
					}

					// Directory-based package for containment (mirrors Go package containment).
					dirPkg := filepath.Dir(fr.RepoRelPath)
					if dirPkg == "." {
						dirPkg = ""
					}
					dirPkgID := graph.NewPackageID(fr.Language, dirPkg)
					packageByID[dirPkgID] = wsPackageRowFor(fr.Language, dirPkg, true, ws, fr.RepoRelPath)
					edgeRows = append(edgeRows, edgeRow(graph.EdgeContains, dirPkgID, string(graph.NodeTypePackage), fileID, string(graph.NodeTypeFile)))

					for _, imp := range jsf.Imports {
						pid := graph.NewPackageID(fr.Language, imp)
						isInt := isJSInternalImport(imp) || ws.IsWorkspacePackage(imp)
						packageByID[pid] = wsPackageRowFor(fr.Language, imp, isInt, ws, "")
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), pid, string(graph.NodeTypePackage)))
					}

					for _, s := range jsf.Symbols {
						q := s.QualifiedName
						semKey := graph.SymbolKey(fr.Language, dirPkg, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						symID := graph.NewSymbolID(fr.Language, dirPkg, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						pkgIDPtr := dirPkgID
						symbolRows = append(symbolRows, artifacts.SymbolRow{
							SymbolID:          symID,
							FileID:            fileID,
							PackageID:         &pkgIDPtr,
							Language:          fr.Language,
							Kind:              s.Kind,
							Name:              s.Name,
							QualifiedName:     q,
							Signature:         nil,
							SemanticKey:       semKey,
							StartLine:         int32(s.StartLine),
							StartCol:          int32(s.StartCol),
							EndLine:           int32(s.EndLine),
							EndCol:            int32(s.EndCol),
							Visibility:        nil,
							Modifiers:         nil,
							ContainerSymbolID: nil,
						})

						edgeRows = append(edgeRows,
							edgeRow(graph.EdgeDefines, fileID, string(graph.NodeTypeFile), symID, string(graph.NodeTypeSymbol)),
							edgeRow(graph.EdgeInFile, symID, string(graph.NodeTypeSymbol), fileID, string(graph.NodeTypeFile)),
						)
					}
				case "python":
					pyf, err := extract.ExtractPythonFile(fr.RepoRelPath, fr.Content)
					if err != nil {
						return fmt.Errorf("extract python %s: %w", fr.RepoRelPath, err)
					}

					// Directory-based package for containment.
					dirPkg := filepath.Dir(fr.RepoRelPath)
					if dirPkg == "." {
						dirPkg = ""
					}
					dirPkgID := graph.NewPackageID("python", dirPkg)
					packageByID[dirPkgID] = wsPackageRowFor("python", dirPkg, true, ws, fr.RepoRelPath)
					edgeRows = append(edgeRows, edgeRow(graph.EdgeContains, dirPkgID, string(graph.NodeTypePackage), fileID, string(graph.NodeTypeFile)))

					for _, imp := range pyf.Imports {
						pid := graph.NewPackageID("python", imp)
						isInt := isPythonInternalImport(imp, pythonPkgDirs) || ws.IsWorkspacePackage(imp)
						packageByID[pid] = wsPackageRowFor("python", imp, isInt, ws, "")
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), pid, string(graph.NodeTypePackage)))
					}

					for _, s := range pyf.Symbols {
						q := s.QualifiedName
						semKey := graph.SymbolKey("python", dirPkg, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						symID := graph.NewSymbolID("python", dirPkg, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						pkgIDPtr := dirPkgID
						symbolRows = append(symbolRows, artifacts.SymbolRow{
							SymbolID:          symID,
							FileID:            fileID,
							PackageID:         &pkgIDPtr,
							Language:          "python",
							Kind:              s.Kind,
							Name:              s.Name,
							QualifiedName:     q,
							Signature:         nil,
							SemanticKey:       semKey,
							StartLine:         int32(s.StartLine),
							StartCol:          int32(s.StartCol),
							EndLine:           int32(s.EndLine),
							EndCol:            int32(s.EndCol),
							Visibility:        nil,
							Modifiers:         nil,
							ContainerSymbolID: nil,
						})

						edgeRows = append(edgeRows,
							edgeRow(graph.EdgeDefines, fileID, string(graph.NodeTypeFile), symID, string(graph.NodeTypeSymbol)),
							edgeRow(graph.EdgeInFile, symID, string(graph.NodeTypeSymbol), fileID, string(graph.NodeTypeFile)),
						)
					}
				case "java":
					// Use pre-parsed result if available (from two-pass).
					jf, ok := javaExtracts[fr.RepoRelPath]
					if !ok {
						var jerr error
						jf, jerr = extract.ExtractJavaFile(fr.RepoRelPath, fr.Content)
						if jerr != nil {
							return fmt.Errorf("extract java %s: %w", fr.RepoRelPath, jerr)
						}
					}

					// Package from the package declaration (or directory fallback).
					javaPkg := jf.PackageName
					if javaPkg == "" {
						javaPkg = filepath.Dir(fr.RepoRelPath)
						if javaPkg == "." {
							javaPkg = ""
						}
					}
					javaPkgID := graph.NewPackageID("java", javaPkg)
					packageByID[javaPkgID] = wsPackageRowFor("java", javaPkg, true, ws, fr.RepoRelPath)
					edgeRows = append(edgeRows, edgeRow(graph.EdgeContains, javaPkgID, string(graph.NodeTypePackage), fileID, string(graph.NodeTypeFile)))

					for _, imp := range jf.Imports {
						pid := graph.NewPackageID("java", imp)
						packageByID[pid] = wsPackageRowFor("java", imp, isJavaInternalImport(imp, javaRepoPkgs), ws, "")
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), pid, string(graph.NodeTypePackage)))
					}

					for _, s := range jf.Symbols {
						q := s.QualifiedName
						semKey := graph.SymbolKey("java", javaPkg, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						symID := graph.NewSymbolID("java", javaPkg, q, fr.RepoRelPath, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
						javaPkgIDPtr := javaPkgID
						symbolRows = append(symbolRows, artifacts.SymbolRow{
							SymbolID:          symID,
							FileID:            fileID,
							PackageID:         &javaPkgIDPtr,
							Language:          "java",
							Kind:              s.Kind,
							Name:              s.Name,
							QualifiedName:     q,
							Signature:         nil,
							SemanticKey:       semKey,
							StartLine:         int32(s.StartLine),
							StartCol:          int32(s.StartCol),
							EndLine:           int32(s.EndLine),
							EndCol:            int32(s.EndCol),
							Visibility:        nil,
							Modifiers:         nil,
							ContainerSymbolID: nil,
						})

						edgeRows = append(edgeRows,
							edgeRow(graph.EdgeDefines, fileID, string(graph.NodeTypeFile), symID, string(graph.NodeTypeSymbol)),
							edgeRow(graph.EdgeInFile, symID, string(graph.NodeTypeSymbol), fileID, string(graph.NodeTypeFile)),
						)
					}
				}
			}

			packageRows := packagesToSortedSlice(packageByID)

			// Write tables (nodes and findings remain empty in this stage).
			if err := artifacts.WriteNodesParquet(filepath.Join(outDir, "nodes.parquet"), nil); err != nil {
				return err
			}
			if err := artifacts.WriteFilesParquet(filepath.Join(outDir, "files.parquet"), fileRows); err != nil {
				return err
			}
			if err := artifacts.WritePackagesParquet(filepath.Join(outDir, "packages.parquet"), packageRows); err != nil {
				return err
			}
			if err := artifacts.WriteSymbolsParquet(filepath.Join(outDir, "symbols.parquet"), symbolRows); err != nil {
				return err
			}
			if err := artifacts.WriteEdgesParquet(filepath.Join(outDir, "edges.parquet"), edgeRows); err != nil {
				return err
			}
			if err := artifacts.WriteFindingsParquet(filepath.Join(outDir, "findings.parquet"), nil); err != nil {
				return err
			}

			lg.Info("index complete", "repo", repoRoot, "out", outDir, "files", len(fileRows), "packages", len(packageRows), "symbols", len(symbolRows), "edges", len(edgeRows))
			return nil
		},
	}

	cmd.Flags().StringVar(&repoRootFlag, "repo", ".", "path to repository root")
	cmd.Flags().StringVar(&out, "out", ".falcon/artifacts", "output directory")

	if err := cmd.MarkFlagDirname("repo"); err != nil {
		return cmd
	}
	if err := cmd.MarkFlagDirname("out"); err != nil {
		return cmd
	}

	cmd.Example = "falcon index --repo . --out .falcon/artifacts"
	cmd.Args = cobra.NoArgs
	cmd.SetHelpTemplate(fmt.Sprintf("%s\n", cmd.HelpTemplate()))
	return cmd
}

func packagesToSortedSlice(m map[string]artifacts.PackageRow) []artifacts.PackageRow {
	if len(m) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]artifacts.PackageRow, 0, len(ids))
	for _, id := range ids {
		out = append(out, m[id])
	}
	return out
}

func packageRowFor(ecosystem, name string, isInternal bool) artifacts.PackageRow {
	// Scope, Version, RootPath, and ManifestPath are intentionally blank here.
	// Callers should use wsPackageRowFor to enrich with workspace metadata when available.
	return artifacts.PackageRow{
		PackageID:    graph.NewPackageID(ecosystem, name),
		Ecosystem:    ecosystem,
		Scope:        "",
		Name:         name,
		Version:      "",
		IsInternal:   isInternal,
		RootPath:     nil,
		ManifestPath: nil,
	}
}

// wsPackageRowFor creates a PackageRow enriched with workspace info when available.
func wsPackageRowFor(ecosystem, name string, isInternal bool, ws *workspace.WorkspaceInfo, repoRelPath string) artifacts.PackageRow {
	row := packageRowFor(ecosystem, name, isInternal)
	if ws.IsEmpty() {
		return row
	}

	// Try to find workspace member by file path first, then by package name.
	var member *workspace.WorkspaceMember
	if repoRelPath != "" {
		member = ws.MemberForPath(repoRelPath)
	}
	if member == nil {
		member = ws.ByPackageName[name]
	}

	if member != nil {
		row.Scope = member.Name
		rootPath := member.RootPath
		row.RootPath = &rootPath
		manifestPath := member.ManifestPath
		row.ManifestPath = &manifestPath
	}
	return row
}

func edgeRow(edgeType graph.EdgeType, srcID, srcType, dstID, dstType string) artifacts.EdgeRow {
	return artifacts.EdgeRow{
		EdgeID:   graph.NewEdgeID(srcID, dstID, edgeType, ""),
		EdgeType: string(edgeType),
		SrcID:    srcID,
		DstID:    dstID,
		SrcType:  srcType,
		DstType:  dstType,
	}
}

func isTestPath(repoRelPath string) bool {
	base := filepath.Base(repoRelPath)
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	if strings.Contains(repoRelPath, "/test/") || strings.Contains(repoRelPath, "/tests/") {
		return true
	}
	return false
}

func isJSInternalImport(target string) bool {
	return strings.HasPrefix(target, "./") ||
		strings.HasPrefix(target, "../") ||
		strings.HasPrefix(target, "~/")
}

func isPythonInternalImport(target string, pkgDirs map[string]bool) bool {
	// Relative imports are always internal.
	if strings.HasPrefix(target, ".") {
		return true
	}
	// Absolute imports: internal if the top-level module matches a repo package dir.
	topLevel := strings.SplitN(target, ".", 2)[0]
	return pkgDirs[topLevel]
}

func isJavaInternalImport(target string, repoPkgs map[string]bool) bool {
	// Try progressively shorter prefixes to match a known repo package.
	parts := strings.Split(target, ".")
	for i := len(parts) - 1; i >= 1; i-- {
		candidate := strings.Join(parts[:i], ".")
		if repoPkgs[candidate] {
			return true
		}
	}
	return false
}

func readGoModulePath(goModPath string) string {
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	// Minimal parse: find first line starting with "module ".
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "module "))
		}
	}
	return ""
}
