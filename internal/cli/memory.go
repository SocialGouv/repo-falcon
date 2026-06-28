package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"repofalcon/internal/logging"
	"repofalcon/internal/memory"
)

func writeFileEnsuringDir(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func defaultMemoryDir(snapshot, memDir string) string {
	if memDir != "" {
		return memDir
	}
	return filepath.Join(snapshot, "memory")
}

// newRememberCmd: `falcon remember` — save the outcome of a graph query into
// work memory, so `falcon reflect` can learn which sources pay off.
func newRememberCmd() *cobra.Command {
	var snapshot, memDir, question, answer, qType, outcome, correction string
	var nodes []string
	cmd := &cobra.Command{
		Use:   "remember",
		Short: "Save a query outcome to work memory (useful/dead_end/corrected)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if question == "" {
				return fmt.Errorf("--question is required")
			}
			if !memory.ValidOutcome(outcome) {
				return fmt.Errorf("--outcome must be one of %s", strings.Join(memory.Outcomes, ", "))
			}
			rec := memory.Record{
				Question:   question,
				Answer:     answer,
				Type:       qType,
				Nodes:      nodes,
				Outcome:    outcome,
				Correction: correction,
				Time:       time.Now().UTC(),
			}
			path, err := memory.Save(defaultMemoryDir(snapshot, memDir), rec)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s outcome to %s\n", outcome, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "snapshot dir (memory lives under it)")
	cmd.Flags().StringVar(&memDir, "memory-dir", "", "override memory directory")
	cmd.Flags().StringVar(&question, "question", "", "the question that was asked (required)")
	cmd.Flags().StringVar(&answer, "answer", "", "the answer given")
	cmd.Flags().StringVar(&qType, "type", "query", "query type: query|path|explain")
	cmd.Flags().StringSliceVar(&nodes, "nodes", nil, "symbol names cited in the answer")
	cmd.Flags().StringVar(&outcome, "outcome", "", "useful|dead_end|corrected (required)")
	cmd.Flags().StringVar(&correction, "correction", "", "the right answer (with --outcome corrected)")
	cmd.Example = "falcon remember --question \"who calls finalizeWorktree\" --nodes RecoverFinalize --outcome useful"
	return cmd
}

// newReflectCmd: `falcon reflect` — aggregate work memory into a deterministic
// LESSONS.md the next session can preload.
func newReflectCmd() *cobra.Command {
	var snapshot, memDir, out, nowStr string
	var halfLife float64
	var minCorr int
	cmd := &cobra.Command{
		Use:   "reflect",
		Short: "Distill work memory into reflections/LESSONS.md (deterministic, no LLM)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			lg := logging.Default()
			recs, err := memory.Load(defaultMemoryDir(snapshot, memDir))
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			if nowStr != "" {
				now, err = time.Parse(time.RFC3339, nowStr)
				if err != nil {
					return fmt.Errorf("--now must be RFC3339: %w", err)
				}
			}
			lessons := memory.Reflect(recs, now, halfLife, minCorr)
			doc := memory.RenderLessons(lessons)

			outPath := out
			if outPath == "" {
				outPath = filepath.Join(snapshot, "reflections", "LESSONS.md")
			}
			if err := writeFileEnsuringDir(outPath, doc); err != nil {
				return err
			}
			lg.Info("reflected", "records", len(recs), "preferred", len(lessons.Preferred),
				"contested", len(lessons.Contested), "dead_ends", len(lessons.DeadEnds), "out", outPath)
			fmt.Fprint(cmd.OutOrStdout(), doc)
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshot, "snapshot", ".falcon/artifacts", "snapshot dir (memory lives under it)")
	cmd.Flags().StringVar(&memDir, "memory-dir", "", "override memory directory")
	cmd.Flags().StringVar(&out, "out", "", "output path (default <snapshot>/reflections/LESSONS.md)")
	cmd.Flags().Float64Var(&halfLife, "half-life-days", 30, "signal weight halves every N days")
	cmd.Flags().IntVar(&minCorr, "min-corroboration", 2, "distinct useful results to prefer a node")
	cmd.Flags().StringVar(&nowStr, "now", "", "reference time (RFC3339) for decay; default now")
	cmd.Example = "falcon reflect --half-life-days 30"
	return cmd
}
