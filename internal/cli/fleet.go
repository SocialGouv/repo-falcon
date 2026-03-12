package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/fleet"
	"repofalcon/internal/logging"
)

func newFleetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Multi-repository commands",
		Long:  "Commands that operate across multiple repositories defined in a fleet manifest (~/.falcon/fleet.json).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newFleetSyncCmd())
	cmd.AddCommand(newFleetMCPCmd())
	cmd.AddCommand(newFleetQueryCmd())
	return cmd
}

// resolveFleetManifest returns a manifest from either --repos flag or --manifest file.
func resolveFleetManifest(manifestPath, reposFlag string) (*fleet.Manifest, error) {
	if reposFlag != "" {
		paths := strings.Split(reposFlag, ",")
		return fleet.ManifestFromPaths(paths), nil
	}
	if manifestPath == "" {
		var err error
		manifestPath, err = fleet.DefaultManifestPath()
		if err != nil {
			return nil, err
		}
	}
	return fleet.LoadManifest(manifestPath)
}

// --- fleet sync ---

func newFleetSyncCmd() *cobra.Command {
	var manifestPath string
	var repos string
	var agents string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Index all repositories in the fleet",
		Long: `Runs 'falcon sync' on every repository listed in the fleet manifest.
Each repo is indexed independently; artifacts stay in each repo's .falcon/artifacts/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			manifest, err := resolveFleetManifest(manifestPath, repos)
			if err != nil {
				return err
			}
			if err := manifest.Validate(); err != nil {
				return err
			}

			lg.Info("fleet sync starting", "repos", len(manifest.Repos))

			for i, repo := range manifest.Repos {
				name := repo.EffectiveName()
				lg.Info("syncing repo", "name", name, "index", i+1, "total", len(manifest.Repos))

				syncCmd := newSyncCmd()
				syncCmd.SetContext(ctx)
				syncArgs := []string{
					"--repo", repo.Path,
					"--out", repo.EffectiveArtifactsDir(),
				}
				if agents != "" {
					syncArgs = append(syncArgs, "--agents", agents)
				} else {
					syncArgs = append(syncArgs, "--agents", "none")
				}
				syncCmd.SetArgs(syncArgs)
				if err := syncCmd.Execute(); err != nil {
					lg.Error("sync failed", "repo", name, "err", err)
					return fmt.Errorf("sync %s: %w", name, err)
				}
				lg.Info("sync complete", "repo", name)
			}

			lg.Info("fleet sync complete", "repos", len(manifest.Repos))
			return nil
		},
	}

	cmd.Flags().StringVar(&manifestPath, "manifest", "", "path to fleet manifest (default: ~/.falcon/fleet.json)")
	cmd.Flags().StringVar(&repos, "repos", "", "comma-separated repo paths (overrides manifest)")
	cmd.Flags().StringVar(&agents, "agents", "none", "agents to configure per repo (default: none)")
	cmd.Args = cobra.NoArgs
	cmd.Example = `  # Sync all repos from manifest
  falcon fleet sync

  # Sync specific repos
  falcon fleet sync --repos ~/apps/web,~/apps/api

  # Sync with agent setup
  falcon fleet sync --agents claude`

	return cmd
}

// --- fleet mcp serve ---

func newFleetMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Fleet MCP server commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newFleetMCPServeCmd())
	return cmd
}

func newFleetMCPServeCmd() *cobra.Command {
	var manifestPath string
	var repos string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start cross-repo MCP server over stdio",
		Long:  "Loads all fleet repo graphs and serves cross-repo MCP tools over stdio (JSON-RPC 2.0).",
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			manifest, err := resolveFleetManifest(manifestPath, repos)
			if err != nil {
				return err
			}

			lg.Info("loading fleet graphs", "repos", len(manifest.Repos))
			fg, err := fleet.LoadFleetGraph(ctx, manifest)
			if err != nil {
				return err
			}

			totalFiles, totalSyms := 0, 0
			for _, rg := range fg.Repos {
				totalFiles += len(rg.Graph.Files)
				totalSyms += len(rg.Graph.Symbols)
			}
			lg.Info("fleet graph loaded",
				"repos", len(fg.Repos),
				"total_files", totalFiles,
				"total_symbols", totalSyms,
			)

			lg.Info("fleet MCP server starting on stdio")
			srv := fleet.NewFleetServer(fg)
			return srv.Serve(os.Stdin, os.Stdout)
		},
	}

	cmd.Flags().StringVar(&manifestPath, "manifest", "", "path to fleet manifest (default: ~/.falcon/fleet.json)")
	cmd.Flags().StringVar(&repos, "repos", "", "comma-separated repo paths (overrides manifest)")
	cmd.Args = cobra.NoArgs

	return cmd
}

// --- fleet query ---

func newFleetQueryCmd() *cobra.Command {
	var manifestPath string
	var repos string
	var format string

	cmd := &cobra.Command{
		Use:   "query <sql>",
		Short: "Run SQL across all fleet Parquet files via DuckDB",
		Long: `Executes ad-hoc SQL against all fleet repos using the DuckDB CLI.

Tables available:
  Per-repo:  <reponame>_files, <reponame>_packages, <reponame>_symbols, ...
  Union:     all_files, all_packages, all_symbols, all_edges, all_findings

Each table includes a _repo column identifying the source repository.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sql := args[0]

			manifest, err := resolveFleetManifest(manifestPath, repos)
			if err != nil {
				return err
			}

			result, err := fleet.RunQuery(manifest, sql, format)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, result)
			return nil
		},
	}

	cmd.Flags().StringVar(&manifestPath, "manifest", "", "path to fleet manifest (default: ~/.falcon/fleet.json)")
	cmd.Flags().StringVar(&repos, "repos", "", "comma-separated repo paths (overrides manifest)")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, json, csv")
	cmd.Example = `  # Find apps using magic-sdk with Next.js
  falcon fleet query "SELECT DISTINCT p._repo FROM all_packages p JOIN all_packages m ON p._repo = m._repo WHERE p.name = 'next' AND m.name = 'magic-sdk'"

  # Find loginWithMagicLink call sites
  falcon fleet query "SELECT _repo, qualified_name, kind FROM all_symbols WHERE qualified_name ILIKE '%loginWithMagicLink%'"

  # File count by language per repo
  falcon fleet query "SELECT _repo, language, COUNT(*) as n FROM all_files GROUP BY _repo, language ORDER BY _repo, n DESC"`

	return cmd
}
