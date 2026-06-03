// Package local runs commands on the host machine.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/sirrobot01/archbench"
)

var _ archbench.Runner = (*Runner)(nil)

// Runner executes a run in a local working directory.
type Runner struct {
	dir   string
	cache archbench.Cache

	cacheDir  string
	ephemeral bool
}

// New returns a runner rooted at dir. An empty dir uses the process directory.
func New(dir string, cache archbench.Cache) *Runner {
	return &Runner{dir: dir, cache: cache}
}

// Prepare resolves the cache directory, creating an ephemeral one when caching
// is disabled.
func (r *Runner) Prepare(context.Context) error {
	if r.cache.Enabled {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		r.cacheDir = filepath.Join(base, "archbench", r.cache.Suite)
		return os.MkdirAll(r.cacheDir, 0o755)
	}

	dir, err := os.MkdirTemp("", "archbench-cache-")
	if err != nil {
		return err
	}
	r.cacheDir = dir
	r.ephemeral = true
	return nil
}

// Cleanup removes the cache directory only when it is ephemeral.
func (r *Runner) Cleanup(context.Context) error {
	if r.ephemeral && r.cacheDir != "" {
		return os.RemoveAll(r.cacheDir)
	}
	return nil
}

func (r *Runner) Capabilities() archbench.Capabilities {
	return archbench.Capabilities{Arch: runtime.GOARCH}
}

// Setup runs target-level provisioning steps in the working directory with the
// cache variables set, so they share the build cache the runs use.
func (r *Runner) Setup(ctx context.Context, steps []string, env map[string]string) error {
	e := r.env(env)
	for _, step := range steps {
		if _, err := r.shell(ctx, step, e); err != nil {
			return fmt.Errorf("setup %q: %w", step, err)
		}
	}
	return nil
}

func (r *Runner) Execute(ctx context.Context, run archbench.Run) (*archbench.Output, error) {
	env := r.env(run.Env)

	for _, step := range run.Setup {
		if _, err := r.shell(ctx, step, env); err != nil {
			return nil, fmt.Errorf("setup %q: %w", step, err)
		}
	}

	out := &archbench.Output{
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
		CPU:       detectCPU(),
		Kernel:    detectKernel(ctx),
		Toolchain: detectToolchain(),
	}

	stdout, err := r.shell(ctx, run.Command, env)
	out.Stdout = stdout
	if err != nil {
		// A cancelled or timed-out run is a runner error, not a test result.
		if ctx.Err() != nil {
			return out, fmt.Errorf("execute %q: %w", run.Command, ctx.Err())
		}
		// A non-zero exit means the tests or benchmarks failed; that is a
		// result, not a runner error.
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			out.ExitCode = ee.ExitCode()
			out.Stderr = string(ee.Stderr)
			return out, nil
		}
		return out, fmt.Errorf("execute %q: %w", run.Command, err)
	}
	return out, nil
}

// env returns the process environment plus the cache variable and any custom
// variables, with cache references expanded.
func (r *Runner) env(custom map[string]string) []string {
	env := append(os.Environ(), archbench.CacheEnv+"="+r.cacheDir)
	for _, k := range sortedKeys(custom) {
		env = append(env, k+"="+archbench.ExpandCache(custom[k], r.cacheDir))
	}
	return env
}

func (r *Runner) shell(ctx context.Context, line string, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", line)
	cmd.Dir = r.dir
	cmd.Env = env

	// Run the command in its own process group and, on cancellation, kill the
	// whole group so child processes (e.g. a `go test` subprocess) don't outlive
	// the timeout and keep the output pipe open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ee, ok := errors.AsType[*exec.ExitError](err); ok && len(ee.Stderr) == 0 {
		ee.Stderr = stderr.Bytes()
	}
	return stdout.String(), err
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func detectToolchain() map[string]string {
	if v := goVersion(); v != "" {
		return map[string]string{"go": v}
	}
	return nil
}

func goVersion() string {
	v := runtime.Version()
	if info, ok := debug.ReadBuildInfo(); ok && info.GoVersion != "" {
		v = info.GoVersion
	}
	return strings.TrimPrefix(v, "go")
}

func detectKernel(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectCPU() string {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	case "linux":
		if out, err := exec.Command("sh", "-c", `grep -m1 'model name' /proc/cpuinfo | cut -d: -f2`).Output(); err == nil {
			if s := strings.TrimSpace(string(out)); s != "" {
				return s
			}
		}
	}
	return ""
}
