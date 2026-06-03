package archbench

import (
	"context"
	"strings"
)

// CacheEnv is the environment variable through which a runner exposes its
// persistent cache directory to commands.
const CacheEnv = "ARCHBENCH_CACHE"

// Runner executes a run against a single target. Implementations run a command
// and capture its raw output; interpreting that output is the parser's job.
type Runner interface {
	Prepare(ctx context.Context) error
	// Setup runs target-level provisioning steps once, after Prepare and before
	// any Execute. The env carries the cache variables the steps should honor
	// (e.g. so a `go mod download` warms the same module cache the runs use).
	Setup(ctx context.Context, steps []string, env map[string]string) error
	Execute(ctx context.Context, run Run) (*Output, error)
	Cleanup(ctx context.Context) error
	Capabilities() Capabilities
}

// SuiteRunner is an optional Runner capability for runners that execute an
// entire Job out-of-process and assemble the RunResult themselves -- e.g. a
// remote `archbench exec` worker -- instead of exposing per-run Execute for the
// engine to drive. The engine calls RunSuite (and skips Prepare/Execute/Cleanup)
// when Capabilities reports Suite. Parsing and environment detection then happen
// where the commands actually run.
type SuiteRunner interface {
	Runner
	RunSuite(ctx context.Context, job Job) (*RunResult, error)
}

// Output is the raw result of running a command on a target, along with the
// environment metadata only the runner can observe.
type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int

	Arch      string
	OS        string
	Kernel    string
	CPU       string
	Toolchain map[string]string

	// Runner optionally names the execution environment, e.g. the GitHub
	// Actions runner label. Most runners leave it empty.
	Runner string
}

// Capabilities describes what a runner supports.
type Capabilities struct {
	Arch             string
	Remote           bool
	SupportsPlatform bool

	// Suite reports that the runner implements SuiteRunner and the engine should
	// delegate the whole Job to RunSuite rather than driving per-run Execute.
	Suite bool
}

// Cache configures build and dependency caching for a runner. When enabled the
// runner keeps a stable cache directory across runs, scoped by Suite. When
// disabled the cache directory is ephemeral, forcing a cold run.
type Cache struct {
	Enabled bool   `json:"enabled"`
	Suite   string `json:"suite"`
}

// ExpandCache replaces references to the cache variable in value with dir.
func ExpandCache(value, dir string) string {
	value = strings.ReplaceAll(value, "${"+CacheEnv+"}", dir)
	value = strings.ReplaceAll(value, "$"+CacheEnv, dir)
	return value
}

// Slug maps a suite name to a filesystem-safe path segment.
func Slug(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}
