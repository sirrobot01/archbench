package ghactions

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/sirrobot01/archbench/spec"
)

func TestRunnerLabelOutsideCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	if got := runnerLabel(); got != "" {
		t.Errorf("runnerLabel outside CI = %q, want empty", got)
	}
}

func TestRunnerLabelInCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("RUNNER_OS", "Linux")
	t.Setenv("RUNNER_ARCH", "X64")
	if got := runnerLabel(); got != "github-actions (Linux/X64)" {
		t.Errorf("runnerLabel = %q", got)
	}
}

// TestExecuteTagsRunner confirms execution behaves like local but stamps the
// runner label onto the output when running inside CI.
func TestExecuteTagsRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("RUNNER_OS", "Linux")
	t.Setenv("RUNNER_ARCH", "ARM64")

	r := New(t.TempDir(), spec.Cache{})
	if err := r.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	t.Cleanup(func() { _ = r.Cleanup(context.Background()) })

	out, err := r.Execute(context.Background(), spec.Run{Command: "echo hi"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Stdout, "hi") {
		t.Errorf("stdout = %q, want it to contain %q", out.Stdout, "hi")
	}
	if out.Runner != "github-actions (Linux/ARM64)" {
		t.Errorf("Runner = %q, want CI label", out.Runner)
	}
}
