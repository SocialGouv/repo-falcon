package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"repofalcon/internal/mcp"
)

// newSymbolCmd: `falcon symbol <name>` — location + callers/callees/references,
// the deterministic CLI counterpart of the falcon_symbol_lookup MCP tool (no
// server session needed).
func newSymbolCmd() *cobra.Command {
	var snapshot, kind string
	cmd := &cobra.Command{
		Use:   "symbol <name>",
		Short: "Look up a symbol: location, callers, callees, references",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.SymbolLookup(args[0], kind))
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Flags().StringVar(&kind, "kind", "", "filter by symbol kind (func, method, type, class, ...)")
	cmd.Example = "falcon symbol finalizeWorktree"
	return cmd
}

// newPathCmd: `falcon path <A> <B>` — shortest call/reference path between two
// symbols.
func newPathCmd() *cobra.Command {
	var snapshot string
	cmd := &cobra.Command{
		Use:   "path <A> <B>",
		Short: "Shortest call/reference path between two symbols",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.ShortestPath(args[0], args[1]))
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Example = "falcon path resolveBackendName detectorResolve"
	return cmd
}

// newCommunitiesCmd: `falcon communities` — deterministic clusters of the
// symbol graph (label propagation, no LLM).
func newCommunitiesCmd() *cobra.Command {
	var snapshot string
	var top int
	cmd := &cobra.Command{
		Use:   "communities",
		Short: "Cluster the symbol graph into communities (deterministic, no LLM)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			labels := mcp.LoadCommunityLabels(snapshot) // optional LLM overlay
			fmt.Fprint(cmd.OutOrStdout(), g.CommunitiesWithLabels(top, labels))
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Flags().IntVar(&top, "top", 25, "number of largest communities to show")
	cmd.Example = "falcon communities --top 20"
	return cmd
}

// newBenchmarkCmd: `falcon benchmark` — estimated token reduction vs raw corpus.
func newBenchmarkCmd() *cobra.Command {
	var snapshot string
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Estimate token reduction of graph queries vs reading the raw corpus",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.Benchmark())
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Example = "falcon benchmark"
	return cmd
}

// newInsightsCmd: `falcon insights` — surprising cross-cluster connections +
// suggested questions (deterministic).
func newInsightsCmd() *cobra.Command {
	var snapshot string
	var top int
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Surprising connections + suggested questions about the codebase",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.Insights(top))
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Flags().IntVar(&top, "top", 10, "number of surprising connections to show")
	cmd.Example = "falcon insights"
	return cmd
}

// newHubsCmd: `falcon hubs` — most connected symbols (degree centrality).
func newHubsCmd() *cobra.Command {
	var snapshot string
	var top int
	cmd := &cobra.Command{
		Use:   "hubs",
		Short: "List the most connected symbols (god nodes / core abstractions)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.Hubs(top))
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Flags().IntVar(&top, "top", 20, "number of hubs to show")
	cmd.Example = "falcon hubs --top 15"
	return cmd
}
