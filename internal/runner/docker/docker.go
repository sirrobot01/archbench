// Package docker runs commands inside a container by delegating to the system
// `docker` CLI. It creates one long-lived container per target, uploads the
// local project into an isolated work directory, runs each run group there with
// `docker exec`, and removes the container on cleanup.
//
// A target may pin `platform` (e.g. linux/amd64) to exercise a non-native
// architecture through the daemon's emulation; the engine flags such runs so
// benchmark timings are reported as untrustworthy.
//
// As with the SSH runner, custom run-group environment may contain secrets, so
// it is written to a 0600 file inside the container over stdin and sourced --
// never passed on the command line, where it would be visible via `ps` or in
// `docker inspect`.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/sirrobot01/archbench"
	"github.com/sirrobot01/archbench/internal/runner/exit"
	"github.com/sirrobot01/archbench/internal/runner/project"
)

// workdir is the in-container directory the project is unpacked into. It is a
// fixed, space-free path so it needs no quoting when handed to `docker`.
const workdir = "/archbench"

var _ archbench.Runner = (*Runner)(nil)

// Runner executes a run inside a container built from the target's image.
type Runner struct {
	target archbench.Target
	dir    string
	cache  archbench.Cache

	container  string
	cacheDir   string
	envSourced bool
}

// New returns a docker runner for target, syncing the local project at dir.
func New(target archbench.Target, dir string, cache archbench.Cache) *Runner {
	return &Runner{target: target, dir: dir, cache: cache}
}

// A docker target runs on the local daemon, so its native architecture is the
// host's. SupportsPlatform lets the engine treat a mismatched `platform` as
// emulation.
func (r *Runner) Capabilities() archbench.Capabilities {
	return archbench.Capabilities{Arch: runtime.GOARCH, SupportsPlatform: true}
}

// Prepare creates and starts the container, then uploads the project into it.
func (r *Runner) Prepare(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker client not found on PATH: %w", err)
	}

	args := []string{"create", "--label", "archbench=1", "-w", workdir}
	if r.target.Platform != "" {
		args = append(args, "--platform", r.target.Platform)
	}
	if r.cache.Enabled {
		host := r.hostCacheDir()
		if err := os.MkdirAll(host, 0o755); err != nil {
			return fmt.Errorf("create cache dir: %w", err)
		}
		r.cacheDir = "/archbench-cache"
		args = append(args, "-v", host+":"+r.cacheDir)
	} else {
		r.cacheDir = workdir + "/.cache"
	}
	// `tail -f /dev/null` keeps the container alive across run groups regardless
	// of the image's default entrypoint or command.
	args = append(args, "--entrypoint", "tail", r.target.Image, "-f", "/dev/null")

	out, stderr, err := r.docker(ctx, args...)
	if err != nil {
		return fmt.Errorf("create container: %w%s", err, withStderr(stderr))
	}
	r.container = strings.TrimSpace(out)
	if r.container == "" {
		return fmt.Errorf("create container: empty id")
	}

	if _, stderr, err := r.docker(ctx, "start", r.container); err != nil {
		return fmt.Errorf("start container: %w%s", err, withStderr(stderr))
	}

	// Ensure the work and (ephemeral) cache directories exist before unpacking.
	if _, stderr, err := r.exec(ctx, "mkdir -p "+shellQuote(workdir)+" "+shellQuote(r.cacheDir)); err != nil {
		return fmt.Errorf("prepare workdir: %w%s", err, withStderr(stderr))
	}
	if err := r.upload(ctx); err != nil {
		return fmt.Errorf("sync project: %w", err)
	}
	return nil
}

// hostCacheDir is the host directory bind-mounted as the container's cache. It
// is scoped by suite and platform: a cache built under one architecture (often
// via emulation) is not reusable under another.
func (r *Runner) hostCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	platform := r.target.Platform
	if platform == "" {
		platform = "native"
	}
	return filepath.Join(base, "archbench", r.cache.Suite, "docker", archbench.Slug(platform))
}

// Setup runs target-level provisioning steps in the work directory. The cache
// env is written to the sourced env file so steps like `go mod download` warm
// the same module cache the runs use.
func (r *Runner) Setup(ctx context.Context, steps []string, env map[string]string) error {
	if len(env) > 0 {
		if err := r.writeEnvFile(ctx, env); err != nil {
			return fmt.Errorf("write env file: %w", err)
		}
		r.envSourced = true
	}
	for _, step := range steps {
		if _, stderr, err := r.exec(ctx, r.inWorkdir(step)); err != nil {
			return fmt.Errorf("setup %q: %w%s", step, err, withStderr(stderr))
		}
	}
	return nil
}

func (r *Runner) Execute(ctx context.Context, run archbench.Run) (*archbench.Output, error) {
	if len(run.Env) > 0 {
		if err := r.writeEnvFile(ctx, run.Env); err != nil {
			return nil, fmt.Errorf("write env file: %w", err)
		}
		r.envSourced = true
	}

	for _, step := range run.Setup {
		if _, stderr, err := r.exec(ctx, r.inWorkdir(step)); err != nil {
			return nil, fmt.Errorf("setup %q: %w%s", step, err, withStderr(stderr))
		}
	}

	out := &archbench.Output{
		Arch:      r.detect(ctx, archCmd),
		OS:        r.detect(ctx, osCmd),
		Kernel:    r.detect(ctx, "uname -r"),
		CPU:       r.detect(ctx, cpuCmd),
		Toolchain: r.detectToolchain(ctx),
	}

	stdout, stderr, err := r.exec(ctx, r.inWorkdir(run.Command))
	out.Stdout = stdout
	out.Stderr = stderr
	if err != nil {
		// A cancelled or timed-out run is a runner error, not a test result.
		if ctx.Err() != nil {
			return out, fmt.Errorf("execute %q: %w", run.Command, ctx.Err())
		}
		// `docker exec` reports 125 for its own (daemon) failures; any other
		// non-zero code is the command's, i.e. the tests or benchmarks failed.
		if code, ok := exit.Result(err, exit.DockerExec); ok {
			out.ExitCode = code
			return out, nil
		}
		return out, fmt.Errorf("execute %q: %w%s", run.Command, err, withStderr(stderr))
	}
	return out, nil
}

// Cleanup force-removes the container, stopping any process still running in it.
func (r *Runner) Cleanup(ctx context.Context) error {
	if r.container != "" {
		_, _, _ = r.docker(ctx, "rm", "-f", r.container)
	}
	return nil
}

// upload packages the local project and extracts it into the work directory by
// streaming a tar.gz into a container-side `tar xzf -`.
func (r *Runner) upload(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", r.container, "tar", "xzf", "-", "-C", workdir)
	setpgKill(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start container tar: %w", err)
	}

	writeErr := make(chan error, 1)
	go func() { writeErr <- project.StreamTar(stdin, r.dir) }()

	waitErr := cmd.Wait()
	if err := <-writeErr; err != nil {
		return fmt.Errorf("package project: %w", err)
	}
	if waitErr != nil {
		return fmt.Errorf("container tar: %w%s", waitErr, withStderr(stderr.String()))
	}
	return nil
}

// docker runs a docker subcommand and returns its stdout and stderr.
func (r *Runner) docker(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	setpgKill(cmd)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err = cmd.Run()
	return so.String(), se.String(), err
}

// exec runs a shell script inside the container via `docker exec`.
func (r *Runner) exec(ctx context.Context, script string) (stdout, stderr string, err error) {
	return r.docker(ctx, "exec", r.container, "sh", "-c", script)
}

// execInput runs a shell script inside the container, feeding input on stdin.
func (r *Runner) execInput(ctx context.Context, script, input string) error {
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", r.container, "sh", "-c", script)
	setpgKill(cmd)
	cmd.Stdin = strings.NewReader(input)
	var se bytes.Buffer
	cmd.Stderr = &se
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w%s", err, withStderr(se.String()))
	}
	return nil
}

// inWorkdir wraps cmd so it runs in the work directory with the cache variable
// set and, when present, the secret env file sourced. The cache path is not
// secret and is set inline; custom env lives in the sourced file.
func (r *Runner) inWorkdir(cmd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export %s=%s; ", archbench.CacheEnv, shellQuote(r.cacheDir))
	if r.envSourced {
		fmt.Fprintf(&b, ". %s; ", shellQuote(r.envFile()))
	}
	fmt.Fprintf(&b, "cd %s && %s", shellQuote(workdir), cmd)
	return b.String()
}

func (r *Runner) envFile() string { return workdir + "/.archbench-env" }

// writeEnvFile writes the custom environment as a sourced shell file with
// restrictive permissions. The values travel over stdin, so they never appear
// in the container's process arguments or `docker inspect` output.
func (r *Runner) writeEnvFile(ctx context.Context, env map[string]string) error {
	var b strings.Builder
	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellQuote(archbench.ExpandCache(env[k], r.cacheDir)))
	}
	return r.execInput(ctx, "umask 077; cat > "+shellQuote(r.envFile()), b.String())
}

func (r *Runner) detect(ctx context.Context, script string) string {
	out, _, err := r.exec(ctx, script)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (r *Runner) detectToolchain(ctx context.Context) map[string]string {
	// "go version go1.24 linux/amd64"
	if v := r.detect(ctx, "go version"); v != "" {
		if fields := strings.Fields(v); len(fields) >= 3 {
			return map[string]string{"go": strings.TrimPrefix(fields[2], "go")}
		}
	}
	return nil
}

// setpgKill runs cmd in its own process group and kills the whole group on
// cancellation. The in-container process may outlive a killed `docker exec`,
// but Cleanup's `docker rm -f` tears the container down regardless.
func setpgKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}

func withStderr(s string) string {
	if s = strings.TrimSpace(s); s != "" {
		return ": " + s
	}
	return ""
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Metadata commands, normalized to Go's GOARCH/GOOS vocabulary.
const (
	archCmd = `uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/'`
	osCmd   = `uname -s | tr '[:upper:]' '[:lower:]'`
	cpuCmd  = `(sysctl -n machdep.cpu.brand_string 2>/dev/null) || ` +
		`(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2) || true`
)

// shellQuote single-quotes s for safe use in a container sh command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
