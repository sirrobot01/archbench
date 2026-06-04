// Package engine runs a spec against its targets and normalizes the results.
package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirrobot01/archbench/internal/runner/docker"
	"github.com/sirrobot01/archbench/internal/runner/ghactions"
	"github.com/sirrobot01/archbench/internal/runner/local"
	"github.com/sirrobot01/archbench/internal/runner/ssh"
	"github.com/sirrobot01/archbench/spec"
)

// Engine runs specs using a set of registered parsers.
type Engine struct {
	parsers *spec.Registry
	dir     string
	cache   bool
}

// New returns an engine that resolves parsers from reg and runs local targets
// rooted at dir. When cache is true, runners persist build caches across runs.
func New(reg *spec.Registry, dir string, cache bool) *Engine {
	return &Engine{parsers: reg, dir: dir, cache: cache}
}

// Run executes the spec against target and returns the result and whether it
// ran under emulation.
func (e *Engine) Run(ctx context.Context, s *spec.Spec, target spec.Target) (*spec.RunResult, bool, error) {
	mode := s.EffectiveMode()

	p, ok := e.parsers.Get(s.Parser)
	if !ok {
		return nil, false, fmt.Errorf("unknown parser %q", s.Parser)
	}
	if !spec.Supports(p, mode) {
		return nil, false, fmt.Errorf("parser %q does not support mode %q", p.Name(), mode)
	}

	cache := spec.Cache{Enabled: e.cache, Suite: spec.Slug(s.Name)}
	r, err := e.runnerFor(target, cache)
	if err != nil {
		return nil, false, err
	}

	// The job is the full unit of work for the target: its setup, env, and runs.
	// Env is the parser's cache wiring overlaid with the target's env; it applies
	// to setup and to every run, with each run's own env layered on top in RunJob.
	job := spec.Job{
		ProtocolVersion: spec.ProtocolVersion,
		Mode:            mode,
		Parser:          s.Parser,
		Setup:           target.Setup,
		Env:             mergeEnv(defaultEnv(s.Parser), target.Env),
		Runs:            s.Runs,
		Cache:           cache,
	}

	// A SuiteRunner (e.g. remote exec) runs the whole job out-of-process and
	// returns the assembled result; every other runner is driven by RunJob.
	var res *spec.RunResult
	if r.Capabilities().Suite {
		sr, ok := r.(spec.SuiteRunner)
		if !ok {
			return nil, false, fmt.Errorf("runner for %q reports Suite but does not implement SuiteRunner", target.Name)
		}
		res, err = sr.RunSuite(ctx, job)
	} else {
		res, err = RunJob(ctx, r, p, job)
	}
	if err != nil {
		return nil, false, fmt.Errorf("run %q: %w", target.Name, err)
	}

	res.Target = target.Name
	return res, emulated(target, r.Capabilities()), nil
}

// RunJob prepares the runner, runs the job's setup and run groups in order, and
// returns the assembled RunResult, owning the runner lifecycle including
// Cleanup. The local engine path and the remote `archbench exec` worker share
// it, so a suite runs identically whether driven in-process or on a host. It
// does not set RunResult.Target; the caller, which knows the target name, does.
func RunJob(ctx context.Context, r spec.Runner, p spec.Parser, job spec.Job) (*spec.RunResult, error) {
	if err := r.Prepare(ctx); err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	// Clean up with an independent context so teardown still runs (e.g. removing
	// the remote workdir) even when ctx was cancelled by a signal or timeout.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = r.Cleanup(cleanupCtx)
	}()

	if len(job.Setup) > 0 {
		if err := r.Setup(ctx, job.Setup, job.Env); err != nil {
			return nil, fmt.Errorf("setup: %w", err)
		}
	}

	started := time.Now()
	res := &spec.RunResult{
		Mode:    job.Mode,
		Started: started,
		Runs:    make([]spec.ScenarioResult, 0, len(job.Runs)),
	}

	for _, specRun := range job.Runs {
		run := specRun
		run.Env = mergeEnv(job.Env, run.Env)

		runStarted := time.Now()
		out, err := r.Execute(ctx, run)
		if err != nil {
			return nil, fmt.Errorf("execute %q: %w", run.Name, err)
		}
		if res.Metadata.Arch == "" && res.Metadata.OS == "" {
			res.Metadata = spec.Metadata{
				Arch:      out.Arch,
				OS:        out.OS,
				Kernel:    out.Kernel,
				CPU:       out.CPU,
				Toolchain: out.Toolchain,
				Runner:    out.Runner,
			}
		} else {
			res.Metadata.Toolchain = mergeEnv(res.Metadata.Toolchain, out.Toolchain)
		}

		parsed, err := p.Parse(job.Mode, out)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", run.Name, err)
		}
		res.Metadata.Toolchain = mergeEnv(res.Metadata.Toolchain, parsed.Toolchain)

		scenario := spec.ScenarioResult{
			Name:            specRun.Name,
			Command:         specRun.Command,
			Started:         runStarted,
			DurationSeconds: time.Since(runStarted).Seconds(),
			ExitCode:        out.ExitCode,
			Benchmarks:      parsed.Benchmarks,
			Tests:           parsed.Tests,
		}
		// Keep the failure reason (e.g. "go: command not found") when a command
		// exits non-zero; for a successful run stderr is noise.
		if out.ExitCode != 0 {
			scenario.Stderr = strings.TrimSpace(out.Stderr)
		}
		res.Runs = append(res.Runs, scenario)
	}
	res.DurationSeconds = time.Since(started).Seconds()
	return res, nil
}

func (e *Engine) runnerFor(t spec.Target, cache spec.Cache) (spec.Runner, error) {
	switch t.Type {
	case spec.TargetLocal:
		return local.New(e.dir, cache), nil
	case spec.TargetSSH:
		return ssh.New(t, e.dir, cache), nil
	case spec.TargetDocker:
		return docker.New(t, e.dir, cache), nil
	case spec.TargetGitHubActions:
		return ghactions.New(e.dir, cache), nil
	default:
		return nil, fmt.Errorf("unknown target type %q", t.Type)
	}
}

// defaultEnv returns cache environment variables a parser's toolchain honors,
// wired to the runner's cache directory. Users may override these in a run
// group's env.
func defaultEnv(parser string) map[string]string {
	switch parser {
	case "go-test":
		return map[string]string{
			"GOCACHE":    "$" + spec.CacheEnv + "/go-build",
			"GOMODCACHE": "$" + spec.CacheEnv + "/go-mod",
		}
	default:
		return nil
	}
}

// emulated reports whether a docker target pins a platform that differs from
// the host architecture, which makes benchmark timings untrustworthy.
func emulated(t spec.Target, caps spec.Capabilities) bool {
	if t.Type == spec.TargetDocker && t.Platform != "" {
		return caps.Arch != "" && !strings.HasSuffix(t.Platform, caps.Arch)
	}
	return false
}

// mergeEnv returns base overlaid with over.
func mergeEnv(base, over map[string]string) map[string]string {
	if len(base) == 0 && len(over) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}
