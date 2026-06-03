package engine

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/sirrobot01/archbench"
)

func TestRunExecutesNamedRunsSequentially(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local runner requires a POSIX shell")
	}

	reg := archbench.NewRegistry()
	reg.Register(stdoutParser{})
	eng := New(reg, t.TempDir(), false)
	spec := &archbench.Spec{
		Name:   "demo",
		Mode:   archbench.ModeBench,
		Parser: "stdout",
		Targets: []archbench.Target{{
			Name: "local",
			Type: archbench.TargetLocal,
		}},
		Runs: []archbench.Run{
			{Name: "first", Command: "printf first"},
			{Name: "second", Command: "printf second"},
		},
	}

	result, emulated, err := eng.Run(context.Background(), spec, spec.Targets[0])
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if emulated {
		t.Fatal("local target reported as emulated")
	}
	if len(result.Runs) != 2 {
		t.Fatalf("runs = %d, want 2: %#v", len(result.Runs), result.Runs)
	}
	if result.Runs[0].Name != "first" || result.Runs[1].Name != "second" {
		t.Fatalf("run order = %#v", result.Runs)
	}
	if result.Runs[0].Benchmarks[0].Name != "first" || result.Runs[1].Benchmarks[0].Name != "second" {
		t.Fatalf("benchmark buckets = %#v", result.Runs)
	}
}

// TestRunTargetSetup confirms target-level setup runs once before the runs and
// its side effects are visible to them.
func TestRunTargetSetup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local runner requires a POSIX shell")
	}

	reg := archbench.NewRegistry()
	reg.Register(stdoutParser{})
	eng := New(reg, t.TempDir(), false)
	spec := &archbench.Spec{
		Name:   "demo",
		Mode:   archbench.ModeBench,
		Parser: "stdout",
		Targets: []archbench.Target{{
			Name:  "local",
			Type:  archbench.TargetLocal,
			Setup: []string{"printf provisioned > marker"},
		}},
		Runs: []archbench.Run{
			{Name: "read", Command: "cat marker"},
		},
	}

	result, _, err := eng.Run(context.Background(), spec, spec.Targets[0])
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := result.Runs[0].Benchmarks[0].Name; got != "provisioned" {
		t.Fatalf("run did not see setup side effect: got %q", got)
	}
}

// TestRunTargetEnv confirms a target's env reaches its runs as a base and that
// a run's own env overrides it.
func TestRunTargetEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local runner requires a POSIX shell")
	}

	reg := archbench.NewRegistry()
	reg.Register(stdoutParser{})
	eng := New(reg, t.TempDir(), false)
	spec := &archbench.Spec{
		Name:   "demo",
		Mode:   archbench.ModeBench,
		Parser: "stdout",
		Targets: []archbench.Target{{
			Name: "local",
			Type: archbench.TargetLocal,
			Env:  map[string]string{"FOO": "base"},
		}},
		Runs: []archbench.Run{
			{Name: "inherits", Command: `printf %s "$FOO"`},
			{Name: "overrides", Command: `printf %s "$FOO"`, Env: map[string]string{"FOO": "run"}},
		},
	}

	result, _, err := eng.Run(context.Background(), spec, spec.Targets[0])
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := result.Runs[0].Benchmarks[0].Name; got != "base" {
		t.Errorf("target env not inherited: got %q, want %q", got, "base")
	}
	if got := result.Runs[1].Benchmarks[0].Name; got != "run" {
		t.Errorf("run env did not override target env: got %q, want %q", got, "run")
	}
}

type stdoutParser struct{}

func (stdoutParser) Name() string {
	return "stdout"
}

func (stdoutParser) Modes() []archbench.Mode {
	return []archbench.Mode{archbench.ModeBench}
}

func (stdoutParser) Parse(_ archbench.Mode, out *archbench.Output) (*archbench.Parsed, error) {
	name := strings.TrimSpace(out.Stdout)
	return &archbench.Parsed{
		Benchmarks: []archbench.Benchmark{{
			Name: name,
			Metrics: map[string]float64{
				archbench.MetricNsPerOp: float64(len(name)),
			},
		}},
	}, nil
}
