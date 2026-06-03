package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench"
	"github.com/sirrobot01/archbench/internal/engine"
	"github.com/sirrobot01/archbench/internal/runner/local"
)

// newExecCmd is the remote worker invoked by exec-mode targets: it reads a Job
// from stdin, runs the suite locally in --dir, and writes the RunResult as JSON
// to stdout. It is hidden because it is an internal protocol endpoint, not a
// user-facing command -- the orchestrator drives it over SSH.
func newExecCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:    "exec",
		Short:  "Run a job from stdin and emit a result (internal remote worker)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var job archbench.Job
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&job); err != nil {
				return fmt.Errorf("decode job: %w", err)
			}
			if job.ProtocolVersion != archbench.ProtocolVersion {
				return fmt.Errorf("unsupported job protocol version %d (this archbench speaks %d); align the orchestrator and host versions",
					job.ProtocolVersion, archbench.ProtocolVersion)
			}

			p, ok := registry().Get(job.Parser)
			if !ok {
				return fmt.Errorf("unknown parser %q", job.Parser)
			}

			res, err := engine.RunJob(cmd.Context(), local.New(dir, job.Cache), p, job)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "project working directory")
	return cmd
}
