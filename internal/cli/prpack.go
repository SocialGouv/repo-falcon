package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/artifacts"
	"repofalcon/internal/duck"
	"repofalcon/internal/git"
	"repofalcon/internal/logging"
	"repofalcon/internal/prpack"
)

func newPRPackCmd() *cobra.Command {
	var repo string
	var snapshot string
	var base string
	var head string
	var out string
	var useDuckDB bool

	cmd := &cobra.Command{
		Use:   "pr-pack",
		Short: "Compute PR context pack (git diff + impact + outputs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			repoRoot := filepath.Clean(strings.TrimSpace(repo))
			snapshotDir := filepath.Clean(strings.TrimSpace(snapshot))
			outDir := strings.TrimSpace(out)
			if outDir == "" {
				outDir = snapshotDir
			}
			outDir = filepath.Clean(outDir)
			if err := artifacts.EnsureDir(outDir); err != nil {
				return err
			}

			changes, err := git.ChangedFiles(repoRoot, base, head)
			if err != nil {
				return err
			}
			changedPaths := make([]string, 0, len(changes))
			changedFiles := make([]prpack.ChangedFile, 0, len(changes))
			for _, c := range changes {
				changedPaths = append(changedPaths, c.Path)
				changedFiles = append(changedFiles, prpack.ChangedFile{Path: c.Path, Status: c.Status, OldPath: c.OldPath})
			}

			files, err := artifacts.ReadFilesParquet(ctx, filepath.Join(snapshotDir, "files.parquet"))
			if err != nil {
				return fmt.Errorf("read snapshot files: %w", err)
			}
			packages, err := artifacts.ReadPackagesParquet(ctx, filepath.Join(snapshotDir, "packages.parquet"))
			if err != nil {
				return fmt.Errorf("read snapshot packages: %w", err)
			}
			symbols, err := artifacts.ReadSymbolsParquet(ctx, filepath.Join(snapshotDir, "symbols.parquet"))
			if err != nil {
				return fmt.Errorf("read snapshot symbols: %w", err)
			}
			edges, err := artifacts.ReadEdgesParquet(ctx, filepath.Join(snapshotDir, "edges.parquet"))
			if err != nil {
				return fmt.Errorf("read snapshot edges: %w", err)
			}
			findings, err := artifacts.ReadFindingsParquet(ctx, filepath.Join(snapshotDir, "findings.parquet"))
			if err != nil {
				return fmt.Errorf("read snapshot findings: %w", err)
			}

			var impact prpack.ImpactResult
			if useDuckDB {
				if !duck.Available() {
					return duck.ErrNotBuiltWithDuckDB
				}
				return fmt.Errorf("duckdb mode is not implemented in this build")
			}
			impact, err = prpack.ComputeImpact(prpack.SnapshotTables{
				Files:    files,
				Packages: packages,
				Symbols:  symbols,
				Edges:    edges,
				Findings: findings,
			}, changedPaths, prpack.ImpactOptions{MaxDepth: 2})
			if err != nil {
				return err
			}

			pack := prpack.BuildContextPack(
				repoRoot,
				snapshotDir,
				base,
				head,
				[]string{"files.parquet", "packages.parquet", "symbols.parquet", "edges.parquet", "findings.parquet", "metadata.json"},
				changedFiles,
				impact,
			)
			if err := prpack.WriteContextPackJSON(filepath.Join(outDir, "pr_context_pack.json"), pack); err != nil {
				return err
			}
			if err := prpack.WriteReviewReportMarkdown(filepath.Join(outDir, "review_report.md"), pack, prpack.ReportOptions{}); err != nil {
				return err
			}

			lg.Info("pr-pack complete",
				"repo", repoRoot,
				"snapshot", snapshotDir,
				"out", outDir,
				"base", base,
				"head", head,
				"changed_files", len(changes),
				"impacted_files", len(pack.ImpactedFiles),
				"impacted_symbols", len(pack.ImpactedSymbols),
				"impacted_packages", len(pack.ImpactedPackages),
				"findings", len(pack.Findings),
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", ".", "path to repository root")
	cmd.Flags().StringVar(&snapshot, "snapshot", "artifacts", "path to existing snapshot artifacts")
	cmd.Flags().StringVar(&base, "base", "", "git base ref")
	cmd.Flags().StringVar(&head, "head", "", "git head ref")
	cmd.Flags().StringVar(&out, "out", "", "output directory (default: --snapshot)")
	cmd.Flags().BoolVar(&useDuckDB, "use-duckdb", false, "use DuckDB (if compiled in) to compute impact")
	_ = cmd.MarkFlagDirname("repo")
	_ = cmd.MarkFlagDirname("snapshot")
	_ = cmd.MarkFlagDirname("out")
	_ = cmd.MarkFlagRequired("base")
	_ = cmd.MarkFlagRequired("head")
	cmd.Args = cobra.NoArgs
	return cmd
}
