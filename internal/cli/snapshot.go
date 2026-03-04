package cli

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"repofalcon/internal/artifacts"
	"repofalcon/internal/logging"
)

func newSnapshotCmd() *cobra.Command {
	var in string
	var out string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Materialize a deterministic snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()

			inDir := in
			if inDir == "" {
				inDir = "artifacts"
			}
			inDir = filepath.Clean(inDir)

			outDir := out
			if outDir == "" {
				outDir = inDir
			}
			outDir = filepath.Clean(outDir)
			if err := artifacts.EnsureDir(outDir); err != nil {
				return err
			}

			counts, err := artifacts.BuildSnapshot(context.Background(), inDir, outDir)
			if err != nil {
				return err
			}

			metaPath := filepath.Join(outDir, "metadata.json")
			meta, err := artifacts.ReadMetadataJSON(metaPath)
			if err != nil {
				// Fall back to minimal metadata if missing.
				meta = artifacts.NewMinimalMetadata(inDir, outDir)
			}
			meta.Kind = "snapshot"
			meta.Artifacts.Path = filepath.Clean(outDir)
			meta.Counts = &artifacts.Counts{
				Nodes:    counts.Nodes,
				Files:    counts.Files,
				Packages: counts.Packages,
				Symbols:  counts.Symbols,
				Findings: counts.Findings,
				Edges:    counts.Edges,
			}
			if err := artifacts.WriteMetadataJSON(metaPath, meta); err != nil {
				return err
			}

			lg.Info("snapshot complete", "in", inDir, "out", outDir, "nodes", counts.Nodes, "edges", counts.Edges)
			return nil
		},
	}

	cmd.Flags().StringVar(&in, "in", "artifacts", "input artifacts directory")
	cmd.Flags().StringVar(&out, "out", "", "output artifacts directory (default: same as --in)")
	_ = cmd.MarkFlagDirname("in")
	_ = cmd.MarkFlagDirname("out")
	cmd.Args = cobra.NoArgs
	return cmd
}
