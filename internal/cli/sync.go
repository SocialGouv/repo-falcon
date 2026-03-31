package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/agentctx"
	"repofalcon/internal/agentsetup"
	"repofalcon/internal/artifacts"
	"repofalcon/internal/logging"
)

func newSyncCmd() *cobra.Command {
	var repoRoot string
	var out string
	var contextOut string
	var agents string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Index, snapshot, and generate agent context in one step",
		Long: `Runs the full pipeline to make a repository ready for coding agents:

  1. falcon index         — parse the repo and extract the code graph
  2. falcon snapshot      — materialize a deterministic snapshot
  3. falcon agent-context — generate a markdown summary for agents
  4. agent setup          — configure your coding agents (interactive or via --agents)

Fully idempotent: run once to set up, run again anytime to refresh.
After running this command, your coding agents will have access to the
code knowledge graph via MCP tools and static context files.`,
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
			lg.Info("step 1/4: indexing repository", "repo", repoDir, "out", outDir)
			indexCmd := newIndexCmd()
			indexCmd.SetContext(ctx)
			indexCmd.SetArgs([]string{"--repo", repoDir, "--out", outDir})
			if err := indexCmd.Execute(); err != nil {
				return err
			}

			// Step 2: snapshot.
			lg.Info("step 2/4: materializing snapshot")
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
			lg.Info("step 3/4: generating agent context", "out", ctxOut)
			if err := agentctx.WriteContext(ctx, outDir, ctxOut, "markdown"); err != nil {
				return err
			}

			// Ensure .falcon/ is in .gitignore.
			if err := agentsetup.EnsureGitignore(repoDir); err != nil {
				lg.Warn("could not update .gitignore", "err", err)
			}

			// Step 4: agent setup.
			selectedAgents := resolveAgents(agents, repoDir, lg)
			if len(selectedAgents) > 0 {
				lg.Info("step 4/4: configuring coding agents")
				for _, id := range selectedAgents {
					if err := configureAgent(id, repoDir, lg); err != nil {
						lg.Warn("failed to configure agent", "agent", string(id), "err", err)
					}
				}
			} else {
				lg.Info("step 4/4: skipping agent setup (no agents selected)")
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
	cmd.Flags().StringVar(&agents, "agents", "", "comma-separated list of agents to configure: claude,cursor,windsurf,copilot,roo,cline (interactive prompt if omitted)")
	_ = cmd.MarkFlagDirname("repo")
	_ = cmd.MarkFlagDirname("out")
	cmd.Args = cobra.NoArgs
	cmd.Aliases = []string{"init"} // "init" kept for backwards compatibility
	cmd.Example = `  # Basic usage — run from your repo root
  falcon sync

  # Non-interactive: specify agents
  falcon sync --agents claude,roo,cline

  # Refresh without agent setup prompt
  falcon sync --agents none`

	return cmd
}

// resolveAgents determines which agents to configure.
// If agents are already configured and no --agents flag is given, it re-uses
// the previously configured agents silently (no interactive prompt).
func resolveAgents(flagVal string, repoRoot string, lg *slog.Logger) []agentsetup.AgentID {
	flagVal = strings.TrimSpace(flagVal)

	// Explicit "none" skips setup.
	if strings.EqualFold(flagVal, "none") {
		return nil
	}

	// If --agents flag was provided, parse it.
	if flagVal != "" {
		ids := agentsetup.ParseAgentIDs(flagVal)
		if len(ids) > 0 {
			return ids
		}
		lg.Warn("no valid agents in --agents flag", "value", flagVal)
		return nil
	}

	// If agents are already configured, re-use them silently.
	if existing := agentsetup.DetectConfiguredAgents(repoRoot); len(existing) > 0 {
		lg.Info("re-configuring previously detected agents", "agents", existing)
		return existing
	}

	// First run with TTY: prompt the user.
	if agentsetup.IsInteractive() {
		ids, err := agentsetup.PromptAgentSelection(os.Stderr, os.Stdin)
		if err != nil {
			lg.Warn("agent selection prompt failed", "err", err)
			return nil
		}
		return ids
	}

	// Non-interactive without --agents: skip.
	lg.Info("non-interactive mode: use --agents flag to configure coding agents")
	return nil
}

// configureAgent dispatches to the appropriate agent configurator.
func configureAgent(id agentsetup.AgentID, repoRoot string, lg *slog.Logger) error {
	switch id {
	case agentsetup.AgentClaude:
		lg.Info("configuring Claude Code", "repo", repoRoot)
		if err := agentsetup.ConfigureClaude(repoRoot); err != nil {
			return fmt.Errorf("claude setup: %w", err)
		}
		lg.Info("  created/updated CLAUDE.md and .mcp.json")
	case agentsetup.AgentCursor:
		lg.Info("configuring Cursor", "repo", repoRoot)
		if err := agentsetup.ConfigureCursor(repoRoot); err != nil {
			return fmt.Errorf("cursor setup: %w", err)
		}
		lg.Info("  created/updated .cursor/rules/falcon.mdc and .cursor/mcp.json")
	case agentsetup.AgentWindsurf:
		lg.Info("configuring Windsurf", "repo", repoRoot)
		if err := agentsetup.ConfigureWindsurf(repoRoot); err != nil {
			return fmt.Errorf("windsurf setup: %w", err)
		}
		lg.Info("  created/updated .windsurfrules and .windsurf/mcp.json")
	case agentsetup.AgentCopilot:
		lg.Info("configuring GitHub Copilot", "repo", repoRoot)
		if err := agentsetup.ConfigureCopilot(repoRoot); err != nil {
			return fmt.Errorf("copilot setup: %w", err)
		}
		lg.Info("  created/updated .github/copilot-instructions.md and .vscode/mcp.json")
	case agentsetup.AgentRoo:
		lg.Info("configuring Roo Code", "repo", repoRoot)
		if err := agentsetup.ConfigureRoo(repoRoot); err != nil {
			return fmt.Errorf("roo setup: %w", err)
		}
		lg.Info("  created .roo/rules/falcon.md and .roo/mcp.json")
	case agentsetup.AgentCline:
		lg.Info("configuring Cline", "repo", repoRoot)
		if err := agentsetup.ConfigureCline(repoRoot); err != nil {
			return fmt.Errorf("cline setup: %w", err)
		}
		lg.Info("  created/updated .clinerules and .cline/mcp_settings.json")
	default:
		return fmt.Errorf("unknown agent: %s", id)
	}
	return nil
}
