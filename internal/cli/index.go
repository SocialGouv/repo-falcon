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

			modulePath := readGoModulePath(filepath.Join(repoRoot, "go.mod"))

			var fileRows []artifacts.FileRow
			var symbolRows []artifacts.SymbolRow
			var edgeRows []artifacts.EdgeRow
			packageByID := map[string]artifacts.PackageRow{}

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
					packageByID[pkgID] = packageRowFor("go", gof.PackageName, true)

					edgeRows = append(edgeRows, edgeRow(graph.EdgeContains, pkgID, string(graph.NodeTypePackage), fileID, string(graph.NodeTypeFile)))

					for _, imp := range gof.Imports {
						impID := graph.NewPackageID("go", imp)
						packageByID[impID] = packageRowFor("go", imp, strings.HasPrefix(imp, modulePath) && modulePath != "")
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
					for _, imp := range extract.ExtractJSImportTargets(fr.Content) {
						pid := graph.NewPackageID(fr.Language, imp)
						packageByID[pid] = packageRowFor(fr.Language, imp, false)
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), pid, string(graph.NodeTypePackage)))
					}
				case "python":
					for _, imp := range extract.ExtractPythonImportTargets(fr.Content) {
						pid := graph.NewPackageID("python", imp)
						packageByID[pid] = packageRowFor("python", imp, false)
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), pid, string(graph.NodeTypePackage)))
					}
				case "java":
					for _, imp := range extract.ExtractJavaImportTargets(fr.Content) {
						pid := graph.NewPackageID("java", imp)
						packageByID[pid] = packageRowFor("java", imp, false)
						edgeRows = append(edgeRows, edgeRow(graph.EdgeImports, fileID, string(graph.NodeTypeFile), pid, string(graph.NodeTypePackage)))
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
	cmd.Flags().StringVar(&out, "out", "artifacts", "output directory")

	if err := cmd.MarkFlagDirname("repo"); err != nil {
		return cmd
	}
	if err := cmd.MarkFlagDirname("out"); err != nil {
		return cmd
	}

	cmd.Example = "falcon index --repo . --out artifacts"
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
	// Scope/version are intentionally blank for now.
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
