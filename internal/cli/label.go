package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"repofalcon/internal/llm"
	"repofalcon/internal/logging"
	"repofalcon/internal/mcp"
	"repofalcon/internal/secure"
)

// newLabelCmd: `falcon label` — OPTIONAL LLM enrichment. Names the deterministic
// communities via a local-first LLM (Ollama by default) and writes them to a
// SEPARATE artifact (community_labels.json). The graph and clustering are never
// modified; this only adds human-readable names on top.
func newLabelCmd() *cobra.Command {
	var snapshot, baseURL, model string
	var top int
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Name communities with a local LLM (optional enrichment; writes community_labels.json)",
		Long: "Label the deterministic symbol-graph communities using an OpenAI-compatible LLM " +
			"(default: local Ollama at http://localhost:11434/v1, model qwen2.5-coder:7b). " +
			"Opt-in and local-first: needs no API key, costs nothing, and never touches the " +
			"deterministic Parquet artifacts — labels live in a separate community_labels.json.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			g, err := mcp.LoadGraph(context.Background(), snapshot)
			if err != nil {
				return err
			}
			client := llm.FromEnv()
			if baseURL != "" {
				client.BaseURL = strings.TrimRight(baseURL, "/")
			}
			if model != "" {
				client.Model = model
			}

			comms := g.ComputeCommunities()
			if top <= 0 || top > len(comms) {
				top = len(comms)
			}
			lg.Info("labeling communities", "backend", client.BaseURL, "model", client.Model, "count", top)

			labels := mcp.LoadCommunityLabels(snapshot)
			if labels == nil {
				labels = map[string]string{}
			}
			ctx := cmd.Context()
			labeled := 0
			for i := 0; i < top; i++ {
				c := comms[i]
				name, err := client.Complete(ctx, labelSystemPrompt, labelUserPrompt(g, c))
				if err != nil {
					// Fail soft: keep deterministic clusters, report and stop.
					lg.Warn("llm labeling failed; keeping unlabeled clusters", "err", err)
					break
				}
				labels[c.RepID] = secure.SanitizeLabel(name)
				labeled++
			}
			if labeled == 0 {
				return fmt.Errorf("no communities labeled (is the LLM backend reachable at %s?)", client.BaseURL)
			}
			if err := mcp.SaveCommunityLabels(snapshot, labels); err != nil {
				return err
			}
			lg.Info("labels written", "file", mcp.CommunityLabelsFile, "labeled", labeled)
			fmt.Fprint(cmd.OutOrStdout(), g.CommunitiesWithLabels(top, labels))
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "path to snapshot artifacts directory")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "OpenAI-compatible base URL (default: env or local Ollama)")
	cmd.Flags().StringVar(&model, "model", "", "model name (default: env or qwen2.5-coder:7b)")
	cmd.Flags().IntVar(&top, "top", 25, "number of largest communities to label")
	cmd.Example = "falcon label --top 20"
	return cmd
}

const labelSystemPrompt = "You name clusters of related code symbols. " +
	"Reply with ONLY a 2-4 word Title Case label naming the concern/domain of the cluster. " +
	"No punctuation, no quotes, no explanation."

func labelUserPrompt(g *mcp.GraphIndex, c mcp.Community) string {
	// Deterministic sample of member names for the prompt.
	names := make([]string, 0, len(c.Members))
	for _, id := range c.Members {
		names = append(names, g.SymbolLabel(id))
	}
	sort.Strings(names)
	if len(names) > 25 {
		names = names[:25]
	}
	return fmt.Sprintf("Core symbol: %s\nMembers (sample): %s\n\nLabel:",
		c.RepLabel, strings.Join(names, ", "))
}
