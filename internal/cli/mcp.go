package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/logging"
	"repofalcon/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}

	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	var snapshot string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server over stdio",
		Long: `Starts a Model Context Protocol (MCP) server over stdin/stdout.
The server loads code graph artifacts from the snapshot directory and exposes
tools for querying the graph: architecture overview, file context, symbol
lookup, package info, and search.

Configure this server in your coding agent's MCP settings to give it
access to the repository's code knowledge graph.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			snapshotDir := filepath.Clean(strings.TrimSpace(snapshot))

			lg.Info("loading graph artifacts", "snapshot", snapshotDir)
			g, err := mcp.LoadGraph(ctx, snapshotDir)
			if err != nil {
				return err
			}
			lg.Info("graph loaded",
				"files", len(g.Files),
				"packages", len(g.Packages),
				"symbols", len(g.Symbols),
				"edges", len(g.Edges),
			)

			lg.Info("MCP server starting on stdio")
			srv := mcp.NewServer(g)
			return srv.Serve(os.Stdin, os.Stdout)
		},
	}

	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	_ = cmd.MarkFlagDirname("snapshot")
	cmd.Args = cobra.NoArgs
	cmd.Example = "falcon mcp serve --snapshot .falcon/artifacts"

	return cmd
}
