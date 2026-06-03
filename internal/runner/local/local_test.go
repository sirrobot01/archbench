package local

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/archbench"
)

func newRunner(t *testing.T) *Runner {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("local runner requires a POSIX shell")
	}
	r := New(t.TempDir(), archbench.Cache{})
	if err := r.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	t.Cleanup(func() { _ = r.Cleanup(context.Background()) })
	return r
}

// TestExecuteTimeout confirms a timed-out run aborts promptly (killing child
// processes) and surfaces an error rather than a misleading empty result.
func TestExecuteTimeout(t *testing.T) {
	r := newRunner(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.Execute(ctx, archbench.Run{Command: "sleep 10"})
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("Execute did not abort promptly: took %s", elapsed)
	}
}

// TestExecuteNonZeroExit confirms a failing command is reported as a result
// (exit code + output), not a runner error.
func TestExecuteNonZeroExit(t *testing.T) {
	r := newRunner(t)

	out, err := r.Execute(context.Background(), archbench.Run{Command: "echo boom; exit 2"})
	if err != nil {
		t.Fatalf("non-zero exit should not be a runner error: %v", err)
	}
	if out.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", out.ExitCode)
	}
	if !strings.Contains(out.Stdout, "boom") {
		t.Errorf("stdout = %q, want it to contain %q", out.Stdout, "boom")
	}
}

// TestSetup confirms target-level setup steps run in the work directory with
// the cache variable and the passed env available, and that their side effects
// persist for later runs.
func TestSetup(t *testing.T) {
	r := newRunner(t)

	err := r.Setup(context.Background(),
		[]string{`echo "cache=$ARCHBENCH_CACHE env=$GOMODCACHE" > marker`},
		map[string]string{"GOMODCACHE": "$" + archbench.CacheEnv + "/go-mod"},
	)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	out, err := r.Execute(context.Background(), archbench.Run{Command: "cat marker"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Stdout, "cache=") || strings.Contains(out.Stdout, "cache= ") {
		t.Errorf("cache variable not set during setup: %q", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "/go-mod") {
		t.Errorf("setup env not expanded: %q", out.Stdout)
	}
}

// TestSetupError confirms a failing setup step surfaces as an error.
func TestSetupError(t *testing.T) {
	r := newRunner(t)

	err := r.Setup(context.Background(), []string{"exit 3"}, nil)
	if err == nil {
		t.Fatal("expected an error from a failing setup step, got nil")
	}
}

// TestExecuteEnv confirms custom env and the cache variable reach the command.
func TestExecuteEnv(t *testing.T) {
	r := newRunner(t)

	out, err := r.Execute(context.Background(), archbench.Run{
		Command: `echo "cache=$ARCHBENCH_CACHE custom=$FOO"`,
		Env:     map[string]string{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Stdout, "custom=bar") {
		t.Errorf("custom env not applied: %q", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "cache=") || strings.Contains(out.Stdout, "cache= ") {
		t.Errorf("cache variable not set: %q", out.Stdout)
	}
}
