package cli

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/agentctx"
	"repofalcon/internal/logging"
)

func newAgentContextCmd() *cobra.Command {
	var snapshot string
	var out string
	var format string

	cmd := &cobra.Command{
		Use:   "agent-context",
		Short: "Generate a code knowledge graph summary for coding agents",
		Long: `Reads Parquet artifacts from a snapshot directory and generates a structured
summary (markdown or JSON) for consumption by coding agents such as Claude Code,
Roo Code, Cline, or Cursor.

The output file can be placed alongside CLAUDE.md, in .roo/rules/, or any
location your agent reads for project context.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			snapshotDir := filepath.Clean(strings.TrimSpace(snapshot))
			outPath := strings.TrimSpace(out)
			if outPath == "" {
				outPath = ".falcon/CONTEXT.md"
			}
			outPath = filepath.Clean(outPath)

			fmt := strings.TrimSpace(strings.ToLower(format))
			if fmt == "" {
				fmt = "markdown"
			}

			if err := agentctx.WriteContext(ctx, snapshotDir, outPath, fmt); err != nil {
				return err
			}

			lg.Info("agent-context complete", "snapshot", snapshotDir, "out", outPath, "format", fmt)
			return nil
		},
	}

	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Flags().StringVar(&out, "out", ".falcon/CONTEXT.md", "output file path")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown or json")
	_ = cmd.MarkFlagDirname("snapshot")
	cmd.Args = cobra.NoArgs
	cmd.Example = "falcon agent-context --snapshot .falcon/artifacts --out .falcon/CONTEXT.md"

	return cmd
}
