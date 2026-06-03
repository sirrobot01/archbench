// Package engine runs a spec against its targets and normalizes the results.
package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirrobot01/archbench"
	"github.com/sirrobot01/archbench/internal/runner/local"
	"github.com/sirrobot01/archbench/internal/runner/ssh"
)

// Engine runs specs using a set of registered parsers.
type Engine struct {
	parsers *archbench.Registry
	dir     string
	cache   bool
}

// New returns an engine that resolves parsers from reg and runs local targets
// rooted at dir. When cache is true, runners persist build caches across runs.
func New(reg *archbench.Registry, dir string, cache bool) *Engine {
	return &Engine{parsers: reg, dir: dir, cache: cache}
}

// Run executes the spec against target and returns the result and whether it
// ran under emulation.
func (e *Engine) Run(ctx context.Context, s *archbench.Spec, target archbench.Target) (*archbench.RunResult, bool, error) {
	mode := s.EffectiveMode()

	p, ok := e.parsers.Get(s.Parser)
	if !ok {
		return nil, false, fmt.Errorf("unknown parser %q", s.Parser)
	}
	if !archbench.Supports(p, mode) {
		return nil, false, fmt.Errorf("parser %q does not support mode %q", p.Name(), mode)
	}

	cache := archbench.Cache{Enabled: e.cache, Suite: archbench.Slug(s.Name)}
	r, err := e.runnerFor(target, cache)
	if err != nil {
		return nil, false, err
	}

	if err := r.Prepare(ctx); err != nil {
		return nil, false, fmt.Errorf("prepare %q: %w", target.Name, err)
	}
	// Clean up with an independent context so teardown still runs (e.g. removing
	// the remote workdir) even when ctx was cancelled by a signal or timeout.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = r.Cleanup(cleanupCtx)
	}()

	started := time.Now()
	res := &archbench.RunResult{
		Target:  target.Name,
		Mode:    mode,
		Started: started,
		Runs:    make([]archbench.ScenarioResult, 0, len(s.Runs)),
	}

	for _, specRun := range s.Runs {
		run := specRun
		run.Env = mergeEnv(defaultEnv(s.Parser), run.Env)

		runStarted := time.Now()
		out, err := r.Execute(ctx, run)
		if err != nil {
			return nil, false, fmt.Errorf("execute %q/%q: %w", target.Name, run.Name, err)
		}
		if res.Metadata.Arch == "" && res.Metadata.OS == "" {
			res.Metadata = archbench.Metadata{
				Arch:      out.Arch,
				OS:        out.OS,
				Kernel:    out.Kernel,
				CPU:       out.CPU,
				Toolchain: out.Toolchain,
			}
		} else {
			res.Metadata.Toolchain = mergeEnv(res.Metadata.Toolchain, out.Toolchain)
		}

		parsed, err := p.Parse(mode, out)
		if err != nil {
			return nil, false, fmt.Errorf("parse %q/%q: %w", target.Name, run.Name, err)
		}
		res.Metadata.Toolchain = mergeEnv(res.Metadata.Toolchain, parsed.Toolchain)

		res.Runs = append(res.Runs, archbench.ScenarioResult{
			Name:            specRun.Name,
			Command:         specRun.Command,
			Started:         runStarted,
			DurationSeconds: time.Since(runStarted).Seconds(),
			ExitCode:        out.ExitCode,
			Benchmarks:      parsed.Benchmarks,
			Tests:           parsed.Tests,
		})
	}
	res.DurationSeconds = time.Since(started).Seconds()
	return res, emulated(target, r.Capabilities()), nil
}

func (e *Engine) runnerFor(t archbench.Target, cache archbench.Cache) (archbench.Runner, error) {
	switch t.Type {
	case archbench.TargetLocal:
		return local.New(e.dir, cache), nil
	case archbench.TargetSSH:
		return ssh.New(t, e.dir, cache), nil
	case archbench.TargetDocker, archbench.TargetGitHubActions:
		return nil, fmt.Errorf("target type %q not yet supported", t.Type)
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
			"GOCACHE":    "$" + archbench.CacheEnv + "/go-build",
			"GOMODCACHE": "$" + archbench.CacheEnv + "/go-mod",
		}
	default:
		return nil
	}
}

// emulated reports whether a docker target pins a platform that differs from
// the host architecture, which makes benchmark timings untrustworthy.
func emulated(t archbench.Target, caps archbench.Capabilities) bool {
	if t.Type == archbench.TargetDocker && t.Platform != "" {
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
