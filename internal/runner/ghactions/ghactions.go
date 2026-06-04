// Package ghactions runs commands on a GitHub Actions runner. Execution is
// local to the runner machine -- the workflow has already checked out the
// project -- so it composes the local runner and only enriches the result with
// the runner's identity (its label and architecture) when running inside CI.
//
// Used together with `archbench generate`, which scaffolds a workflow that
// invokes `archbench run` against these targets on the chosen runners.
package ghactions

import (
	"context"
	"os"

	"github.com/sirrobot01/archbench/internal/runner/local"
	"github.com/sirrobot01/archbench/spec"
)

var _ spec.Runner = (*Runner)(nil)

// Runner executes a run on the current machine, tagging output with GitHub
// Actions runner metadata. Outside CI it behaves exactly like the local runner.
type Runner struct {
	*local.Runner
}

// New returns a github-actions runner rooted at dir.
func New(dir string, cache spec.Cache) *Runner {
	return &Runner{Runner: local.New(dir, cache)}
}

// Execute delegates to the local runner, then stamps the GitHub runner label
// onto the output so reports record which CI machine produced the numbers.
func (r *Runner) Execute(ctx context.Context, run spec.Run) (*spec.Output, error) {
	out, err := r.Runner.Execute(ctx, run)
	if out != nil {
		out.Runner = runnerLabel()
	}
	return out, err
}

// runnerLabel describes the GitHub Actions runner from the environment GitHub
// injects. It is empty outside CI, leaving the result indistinguishable from a
// plain local run.
func runnerLabel() string {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return ""
	}
	osName, arch := os.Getenv("RUNNER_OS"), os.Getenv("RUNNER_ARCH")
	switch {
	case osName != "" && arch != "":
		return "github-actions (" + osName + "/" + arch + ")"
	case osName != "":
		return "github-actions (" + osName + ")"
	default:
		return "github-actions"
	}
}
