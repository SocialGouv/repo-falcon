package cli

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/agentctx"
	"repofalcon/internal/artifacts"
	"repofalcon/internal/logging"
)

func newInitCmd() *cobra.Command {
	var repoRoot string
	var out string
	var contextOut string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Index, snapshot, and generate agent context in one step",
		Long: `Runs the full pipeline to make a repository ready for coding agents:

  1. falcon index   — parse the repo and extract the code graph
  2. falcon snapshot — materialize a deterministic snapshot
  3. falcon agent-context — generate a markdown summary for agents

After running this command, configure your agent's MCP server to point at
the artifacts directory, or use the generated context file directly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			repoDir := filepath.Clean(strings.TrimSpace(repoRoot))
			outDir := filepath.Clean(strings.TrimSpace(out))
			if err := artifacts.EnsureDir(outDir); err != nil {
				return err
			}

			// Step 1: index.
			lg.Info("step 1/3: indexing repository", "repo", repoDir, "out", outDir)
			indexCmd := newIndexCmd()
			indexCmd.SetContext(ctx)
			indexCmd.SetArgs([]string{"--repo", repoDir, "--out", outDir})
			if err := indexCmd.Execute(); err != nil {
				return err
			}

			// Step 2: snapshot.
			lg.Info("step 2/3: materializing snapshot")
			snapshotCmd := newSnapshotCmd()
			snapshotCmd.SetContext(ctx)
			snapshotCmd.SetArgs([]string{"--in", outDir, "--out", outDir})
			if err := snapshotCmd.Execute(); err != nil {
				return err
			}

			// Step 3: agent-context.
			ctxOut := strings.TrimSpace(contextOut)
			if ctxOut == "" {
				ctxOut = filepath.Join(repoDir, ".falcon", "CONTEXT.md")
			}
			lg.Info("step 3/3: generating agent context", "out", ctxOut)
			if err := agentctx.WriteContext(ctx, outDir, ctxOut, "markdown"); err != nil {
				return err
			}

			lg.Info("init complete — repository is ready for coding agents",
				"artifacts", outDir,
				"context", ctxOut,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo", ".", "path to repository root")
	cmd.Flags().StringVar(&out, "out", ".falcon/artifacts", "output directory for artifacts")
	cmd.Flags().StringVar(&contextOut, "context-out", "", "output path for context file (default: <repo>/.falcon/CONTEXT.md)")
	_ = cmd.MarkFlagDirname("repo")
	_ = cmd.MarkFlagDirname("out")
	cmd.Args = cobra.NoArgs
	cmd.Example = `  # Basic usage
  falcon init --repo .

  # Custom output paths
  falcon init --repo /path/to/project --out /path/to/project/.falcon/artifacts --context-out /path/to/project/AGENTS.md`

	return cmd
}
