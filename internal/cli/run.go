package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench"
	"github.com/sirrobot01/archbench/internal/engine"
	"github.com/sirrobot01/archbench/internal/ui"
)

func newRunCmd() *cobra.Command {
	var (
		specPath    string
		dir         string
		outDir      string
		only        string
		noCache     bool
		timeout     time.Duration
		concurrency int
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the suite against all targets, or one with --target",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := archbench.LoadSpec(specPath)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}

			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			if concurrency < 1 {
				return fmt.Errorf("--concurrency must be at least 1")
			}

			targets := selectTargets(s.Targets, only)
			if only != "" && len(targets) == 0 {
				return fmt.Errorf("no target named %q", only)
			}

			eng := engine.New(registry(), dir, !noCache)
			var results []targetRun
			err = ui.Spinner(ctx, cmd.ErrOrStderr(), runTitle(targets, concurrency), func(spinnerCtx context.Context) error {
				var runErr error
				results, runErr = runTargets(spinnerCtx, targets, concurrency, func(runCtx context.Context, target archbench.Target) (*targetRun, error) {
					var out targetRun
					var runErr error
					out.Result, out.Emulated, runErr = eng.Run(runCtx, s, target)
					return &out, runErr
				})
				return runErr
			})
			if err != nil {
				return err
			}

			for _, result := range results {
				if result.Emulated && result.Result.Mode == archbench.ModeBench {
					cmd.PrintErrf("⚠️  target %q runs under emulation — benchmark timings are not trustworthy\n", result.Target.Name)
				}

				path := filepath.Join(outDir, result.Target.Name+".json")
				if err := writeJSON(path, result.Result); err != nil {
					return err
				}
				engine.Terminal(cmd, result.Result)
				cmd.PrintErrf("→ %s\n", path)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&specPath, "spec", "s", archbench.DefaultSpecFile, "path to spec file")
	cmd.Flags().StringVar(&dir, "dir", ".", "project working directory")
	cmd.Flags().StringVarP(&outDir, "out", "o", "archbench-results", "directory for result artifacts")
	cmd.Flags().StringVarP(&only, "target", "t", "", "run only this target")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "force a cold run with an ephemeral cache")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "abort the run after this duration (0 = no timeout)")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 1, "maximum number of targets to run at once")
	return cmd
}

type targetRun struct {
	Target   archbench.Target
	Result   *archbench.RunResult
	Emulated bool
}

type targetRunner func(context.Context, archbench.Target) (*targetRun, error)

func selectTargets(targets []archbench.Target, only string) []archbench.Target {
	selected := make([]archbench.Target, 0, len(targets))
	for _, t := range targets {
		if only != "" && t.Name != only {
			continue
		}
		selected = append(selected, t)
	}
	return selected
}

func runTitle(targets []archbench.Target, concurrency int) string {
	if len(targets) == 1 {
		return fmt.Sprintf("running %s", targets[0].Name)
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}
	return fmt.Sprintf("running %d targets with concurrency %d", len(targets), concurrency)
}

func runTargets(ctx context.Context, targets []archbench.Target, concurrency int, runner targetRunner) ([]targetRun, error) {
	if concurrency < 1 {
		return nil, fmt.Errorf("concurrency must be at least 1")
	}
	if len(targets) == 0 {
		return nil, nil
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]targetRun, len(targets))
	jobs := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range jobs {
			target := targets[idx]
			run, err := runner(ctx, target)
			if err != nil {
				select {
				case errCh <- err:
					cancel()
				default:
				}
				continue
			}
			if run == nil || run.Result == nil {
				select {
				case errCh <- fmt.Errorf("target %q returned no result", target.Name):
					cancel()
				default:
				}
				continue
			}
			run.Target = target
			results[idx] = *run
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

send:
	for i := range targets {
		select {
		case <-ctx.Done():
			break send
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
