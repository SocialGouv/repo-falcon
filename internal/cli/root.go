package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/appinfo"
	"repofalcon/internal/logging"
)

type RootFlags struct {
	LogLevel string
}

// NewRootCommand constructs the root `falcon` CLI.
func NewRootCommand() *cobra.Command {
	var rf RootFlags

	cmd := &cobra.Command{
		Use:           "falcon",
		Short:         "RepoFalcon CLI",
		Long:          "RepoFalcon — index your repository and generate a code knowledge graph for coding agents.",
		Version:       appinfo.FullVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			lvl := strings.TrimSpace(rf.LogLevel)
			lg, err := logging.New(lvl)
			if err != nil {
				return err
			}
			logging.SetDefault(lg)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&rf.LogLevel, "log-level", "info", "log level (debug, info, warn, error)")

	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newIndexCmd())
	cmd.AddCommand(newSnapshotCmd())
	cmd.AddCommand(newPRPackCmd())
	cmd.AddCommand(newAgentContextCmd())
	cmd.AddCommand(newMCPCmd())

	// Cobra's default help/usage outputs include timestamps only if we log them; we don't.
	cmd.SetContext(context.Background())

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return fmt.Errorf("%w\n\n%s", err, c.UsageString())
	})

	return cmd
}
