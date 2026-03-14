package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/agentsetup"
	"repofalcon/internal/logging"
)

func newAgentSetupCmd() *cobra.Command {
	var repoRoot string
	var agents string

	cmd := &cobra.Command{
		Use:   "agent-setup",
		Short: "Configure coding agents to use the falcon code knowledge graph",
		Long: `Set up one or more coding agents (Claude Code, Cursor, Windsurf, GitHub Copilot,
Roo Code, Cline) so they can access the falcon MCP tools and static context.

This creates/updates agent-specific instruction files and MCP server
configuration without re-running the full index/snapshot pipeline.

Use this after "falcon sync" to add or reconfigure agents, or to integrate
a new agent into an already-indexed repository.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()

			repoDir := filepath.Clean(strings.TrimSpace(repoRoot))

			// Ensure .falcon/ is in .gitignore.
			if err := agentsetup.EnsureGitignore(repoDir); err != nil {
				lg.Warn("could not update .gitignore", "err", err)
			}

			selectedAgents := resolveAgents(agents, repoDir, lg)
			if len(selectedAgents) == 0 {
				lg.Info("no agents selected — nothing to do")
				return nil
			}

			lg.Info("configuring coding agents")
			for _, id := range selectedAgents {
				if err := configureAgent(id, repoDir, "falcon", lg); err != nil {
					return fmt.Errorf("agent %s: %w", id, err)
				}
			}

			lg.Info("agent setup complete")
			return nil
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo", ".", "path to repository root")
	cmd.Flags().StringVar(&agents, "agents", "", "comma-separated list of agents to configure: claude,cursor,windsurf,copilot,roo,cline (interactive prompt if omitted)")
	_ = cmd.MarkFlagDirname("repo")
	cmd.Args = cobra.NoArgs
	cmd.Example = `  # Interactive: select agents with arrow keys
  falcon agent-setup

  # Non-interactive: specify agents directly
  falcon agent-setup --agents claude,cursor

  # Reconfigure a single agent
  falcon agent-setup --agents claude`

	return cmd
}
