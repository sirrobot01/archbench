// Package ssh runs commands on a remote host by delegating to the system
// OpenSSH client. This inherits the user's ~/.ssh/config (host aliases,
// identities, ProxyJump/ProxyCommand, multiplexing), agent, and known_hosts.
//
// It packages the local project, uploads it to an isolated work directory,
// runs the command there, captures the output, and removes the directory on
// cleanup. Host keys are verified by ssh (accept-new by default); set
// ARCHBENCH_SSH_INSECURE=1 to skip verification on trusted networks.
//
// Custom run-group environment may contain secrets, so it is written to a 0600
// file inside the work directory over stdin and sourced -- never passed on the
// command line, where it would be visible to other users via ps.
package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirrobot01/archbench"
)

const insecureEnv = "ARCHBENCH_SSH_INSECURE"

var _ archbench.Runner = (*Runner)(nil)

// Runner executes a run on a remote host over SSH.
type Runner struct {
	target archbench.Target
	dir    string
	cache  archbench.Cache

	workdir    string
	cacheDir   string
	envSourced bool
}

// New returns an SSH runner for target, syncing the local project at dir.
func New(target archbench.Target, dir string, cache archbench.Cache) *Runner {
	return &Runner{target: target, dir: dir, cache: cache}
}

func (r *Runner) Capabilities() archbench.Capabilities {
	return archbench.Capabilities{Remote: true}
}

// Prepare creates an isolated remote work directory and uploads the project.
func (r *Runner) Prepare(ctx context.Context) error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("ssh client not found on PATH: %w", err)
	}
	if insecure() {
		fmt.Fprintf(os.Stderr, "⚠️  host-key verification DISABLED for %s (%s set)\n", r.target.Host, insecureEnv)
	}

	wd, _, err := r.run(ctx, "mktemp -d -t archbench.XXXXXX")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	r.workdir = strings.TrimSpace(wd)
	if r.workdir == "" {
		return fmt.Errorf("create workdir: empty path")
	}

	if err := r.setupCache(ctx); err != nil {
		return fmt.Errorf("setup cache: %w", err)
	}
	if err := r.upload(ctx); err != nil {
		return fmt.Errorf("sync project: %w", err)
	}
	return nil
}

// setupCache resolves the remote cache directory. When caching is enabled it is
// a stable, per-suite directory under the home directory; otherwise it lives
// inside the work directory and is removed with it.
func (r *Runner) setupCache(ctx context.Context) error {
	if r.cache.Enabled {
		out, _, err := r.run(ctx, `d="$HOME/.cache/archbench/`+r.cache.Suite+`"; mkdir -p "$d" && printf %s "$d"`)
		if err != nil {
			return err
		}
		r.cacheDir = strings.TrimSpace(out)
		return nil
	}
	r.cacheDir = r.workdir + "/.cache"
	_, _, err := r.run(ctx, "mkdir -p "+shellQuote(r.cacheDir))
	return err
}

func (r *Runner) Execute(ctx context.Context, run archbench.Run) (*archbench.Output, error) {
	if len(run.Env) > 0 {
		if err := r.writeEnvFile(ctx, run.Env); err != nil {
			return nil, fmt.Errorf("write env file: %w", err)
		}
		r.envSourced = true
	}

	for _, step := range run.Setup {
		if _, stderr, err := r.run(ctx, r.inWorkdir(step)); err != nil {
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

	stdout, stderr, err := r.run(ctx, r.inWorkdir(run.Command))
	out.Stdout = stdout
	out.Stderr = stderr
	if err != nil {
		// A cancelled or timed-out run is a runner error, not a test result.
		if ctx.Err() != nil {
			return out, fmt.Errorf("execute %q: %w", run.Command, ctx.Err())
		}
		// ssh reports 255 for its own (connection) failures; any other exit
		// code is the remote command's, i.e. the tests or benchmarks failed.
		if code := exitCode(err); code >= 0 && code != 255 {
			out.ExitCode = code
			return out, nil
		}
		return out, fmt.Errorf("execute %q: %w%s", run.Command, err, withStderr(stderr))
	}
	return out, nil
}

// Cleanup removes the remote work directory.
func (r *Runner) Cleanup(ctx context.Context) error {
	if strings.HasPrefix(r.workdir, "/") && len(r.workdir) > 1 {
		_, _, _ = r.run(ctx, "rm -rf "+shellQuote(r.workdir))
	}
	return nil
}

// upload packages the local project and extracts it into the work directory by
// streaming a tar.gz into a remote `tar xzf -`.
func (r *Runner) upload(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ssh", append(r.sshArgs(), "tar xzf - -C "+shellQuote(r.workdir))...)
	setpgKill(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start remote tar: %w", err)
	}

	writeErr := make(chan error, 1)
	go func() { writeErr <- streamTar(stdin, r.dir) }()

	waitErr := cmd.Wait()
	if err := <-writeErr; err != nil {
		return fmt.Errorf("package project: %w", err)
	}
	if waitErr != nil {
		return fmt.Errorf("remote tar: %w%s", waitErr, withStderr(stderr.String()))
	}
	return nil
}

// run executes a single remote command via ssh and returns stdout and stderr.
func (r *Runner) run(ctx context.Context, remote string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "ssh", append(r.sshArgs(), remote)...)
	setpgKill(cmd)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err = cmd.Run()
	return so.String(), se.String(), err
}

// runInput executes a remote command feeding input on stdin, discarding stdout.
func (r *Runner) runInput(ctx context.Context, remote, input string) error {
	cmd := exec.CommandContext(ctx, "ssh", append(r.sshArgs(), remote)...)
	setpgKill(cmd)
	cmd.Stdin = strings.NewReader(input)
	var se bytes.Buffer
	cmd.Stderr = &se
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w%s", err, withStderr(se.String()))
	}
	return nil
}

// sshArgs builds the ssh flags. Explicit target fields override the user's
// ssh config; anything unset is left for ssh to resolve.
func (r *Runner) sshArgs() []string {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=15"}
	if insecure() {
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	} else {
		args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	}
	if r.target.Port != 0 {
		args = append(args, "-p", strconv.Itoa(r.target.Port))
	}
	if r.target.Key != "" {
		args = append(args, "-i", r.target.Key)
	}
	if r.target.ProxyJump != "" {
		args = append(args, "-J", r.target.ProxyJump)
	}
	return append(args, destination(r.target))
}

func destination(t archbench.Target) string {
	if t.User != "" {
		return t.User + "@" + t.Host
	}
	return t.Host
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
	fmt.Fprintf(&b, "cd %s && %s", shellQuote(r.workdir), cmd)
	return b.String()
}

func (r *Runner) envFile() string { return r.workdir + "/.archbench-env" }

// writeEnvFile writes the custom environment as a sourced shell file with
// restrictive permissions. The values travel over stdin, so they never appear
// in the remote process arguments.
func (r *Runner) writeEnvFile(ctx context.Context, env map[string]string) error {
	var b strings.Builder
	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellQuote(archbench.ExpandCache(env[k], r.cacheDir)))
	}
	return r.runInput(ctx, "umask 077; cat > "+shellQuote(r.envFile()), b.String())
}

func (r *Runner) detect(ctx context.Context, remote string) string {
	out, _, err := r.run(ctx, remote)
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
// cancellation, so remote-side child processes don't outlive a timeout.
func setpgKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode()
	}
	return -1
}

func insecure() bool {
	v := os.Getenv(insecureEnv)
	return v == "1" || v == "true"
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

// shellQuote single-quotes s for safe use in a remote sh command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
