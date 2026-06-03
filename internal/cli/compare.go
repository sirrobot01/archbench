package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirrobot01/archbench/internal/engine"
	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench"
)

func newCompareCmd() *cobra.Command {
	var threshold float64
	cmd := &cobra.Command{
		Use:   "compare <baseline.json> <candidate.json>",
		Short: "Compare two result artifacts",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := readResult(args[0])
			if err != nil {
				return err
			}
			b, err := readResult(args[1])
			if err != nil {
				return err
			}
			if err := engine.Compare(cmd, a, b); err != nil {
				return err
			}
			if threshold <= 0 {
				return nil
			}
			regs := engine.BenchRegressions(a, b, threshold)
			if len(regs) == 0 {
				return nil
			}
			cmd.PrintErrf("\n%d benchmark(s) regressed beyond %.1f%%:\n", len(regs), threshold)
			for _, r := range regs {
				cmd.PrintErrf("  %s/%s: %.0f → %.0f ns/op (+%.1f%%)\n",
					r.Run, r.Benchmark, r.Baseline, r.Candidate, r.Percent)
			}
			return fmt.Errorf("benchmark regression threshold exceeded")
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 0,
		"fail if any benchmark's ns/op regresses by more than this percent (0 = never fail)")
	return cmd
}

func readResult(path string) (*archbench.RunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r archbench.RunResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse result %q: %w", path, err)
	}
	return &r, nil
}
