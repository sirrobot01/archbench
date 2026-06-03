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
	return &cobra.Command{
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
			return engine.Compare(cmd, a, b)
		},
	}
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
