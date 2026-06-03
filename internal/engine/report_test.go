package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench"
)

func TestCompareBench(t *testing.T) {
	cmd, out := captureCommand()
	err := Compare(cmd,
		benchResult("arm64", 100),
		benchResult("amd64", 150),
	)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	got := out.String()
	for _, want := range []string{"BenchmarkRead", "100", "150", "+50.0% slower"} {
		if !strings.Contains(got, want) {
			t.Errorf("compare output missing %q:\n%s", want, got)
		}
	}
}

func TestCompareBenchReportsNewAndRemovedItems(t *testing.T) {
	baseline := benchResult("arm64", 100)
	baseline.Runs[0].Benchmarks = append(baseline.Runs[0].Benchmarks, archbench.Benchmark{
		Name:    "BenchmarkRemoved",
		Metrics: map[string]float64{archbench.MetricNsPerOp: 200},
	})
	baseline.Runs = append(baseline.Runs, archbench.ScenarioResult{
		Name: "removed-run",
		Benchmarks: []archbench.Benchmark{{
			Name:    "BenchmarkOnlyBaseline",
			Metrics: map[string]float64{archbench.MetricNsPerOp: 300},
		}},
	})

	candidate := benchResult("amd64", 100)
	candidate.Runs[0].Benchmarks = append(candidate.Runs[0].Benchmarks, archbench.Benchmark{
		Name:    "BenchmarkNew",
		Metrics: map[string]float64{archbench.MetricNsPerOp: 400},
	})
	candidate.Runs = append(candidate.Runs, archbench.ScenarioResult{
		Name: "new-run",
		Benchmarks: []archbench.Benchmark{{
			Name:    "BenchmarkOnlyCandidate",
			Metrics: map[string]float64{archbench.MetricNsPerOp: 500},
		}},
	})

	cmd, out := captureCommand()
	err := Compare(cmd, baseline, candidate)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	got := out.String()
	for _, want := range []string{"BenchmarkNew", "(new)", "BenchmarkRemoved", "(removed)", "(new run)", "(removed run)"} {
		if !strings.Contains(got, want) {
			t.Errorf("compare output missing %q:\n%s", want, got)
		}
	}
}

func TestBenchRegressions(t *testing.T) {
	// 100 -> 150 ns/op is a +50% regression on the shared "stream" run.
	baseline := benchResult("arm64", 100)
	candidate := benchResult("amd64", 150)

	// A benchmark that improved is not a regression.
	baseline.Runs[0].Benchmarks = append(baseline.Runs[0].Benchmarks, archbench.Benchmark{
		Name:    "BenchmarkFaster",
		Metrics: map[string]float64{archbench.MetricNsPerOp: 200},
	})
	candidate.Runs[0].Benchmarks = append(candidate.Runs[0].Benchmarks, archbench.Benchmark{
		Name:    "BenchmarkFaster",
		Metrics: map[string]float64{archbench.MetricNsPerOp: 100},
	})

	// Below the threshold, +50% is reported but +10% is not.
	regs := BenchRegressions(baseline, candidate, 25)
	if len(regs) != 1 {
		t.Fatalf("regressions = %d, want 1: %#v", len(regs), regs)
	}
	if regs[0].Benchmark != "BenchmarkRead" || regs[0].Run != "stream" {
		t.Errorf("unexpected regression: %#v", regs[0])
	}
	if regs[0].Percent != 50 {
		t.Errorf("percent = %v, want 50", regs[0].Percent)
	}

	// A higher threshold tolerates the same regression.
	if regs := BenchRegressions(baseline, candidate, 75); len(regs) != 0 {
		t.Errorf("regressions above threshold = %#v, want none", regs)
	}
}

// TestBenchRegressionsSkipsUnmatched confirms a benchmark present on only one
// side is treated as new/removed rather than a regression.
func TestBenchRegressionsSkipsUnmatched(t *testing.T) {
	baseline := benchResult("arm64", 100)
	candidate := benchResult("amd64", 100)
	candidate.Runs[0].Benchmarks = append(candidate.Runs[0].Benchmarks, archbench.Benchmark{
		Name:    "BenchmarkNew",
		Metrics: map[string]float64{archbench.MetricNsPerOp: 9999},
	})

	if regs := BenchRegressions(baseline, candidate, 1); len(regs) != 0 {
		t.Errorf("regressions = %#v, want none (new benchmark is not a regression)", regs)
	}
}

func TestCompareTestDivergence(t *testing.T) {
	cmd, out := captureCommand()
	err := Compare(cmd,
		testResult("arm64", archbench.StatusPass),
		testResult("amd64", archbench.StatusFail),
	)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	got := out.String()
	for _, want := range []string{"TestRead", "pass", "fail", "1 test(s) diverge"} {
		if !strings.Contains(got, want) {
			t.Errorf("compare output missing %q:\n%s", want, got)
		}
	}
}

func TestCompareTestReportsNewAndRemovedRuns(t *testing.T) {
	baseline := testResult("arm64", archbench.StatusPass)
	baseline.Runs = append(baseline.Runs, archbench.ScenarioResult{
		Name: "removed-run",
		Tests: []archbench.Test{{
			Name:   "TestOnlyBaseline",
			Status: archbench.StatusPass,
		}},
	})

	candidate := testResult("amd64", archbench.StatusPass)
	candidate.Runs = append(candidate.Runs, archbench.ScenarioResult{
		Name: "new-run",
		Tests: []archbench.Test{{
			Name:   "TestOnlyCandidate",
			Status: archbench.StatusPass,
		}},
	})

	cmd, out := captureCommand()
	err := Compare(cmd, baseline, candidate)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	got := out.String()
	for _, want := range []string{"TestOnlyCandidate", "(new run)", "TestOnlyBaseline", "(removed run)"} {
		if !strings.Contains(got, want) {
			t.Errorf("compare output missing %q:\n%s", want, got)
		}
	}
}

// TestTerminalSurfacesFailedRun confirms a run that exited non-zero with no
// parsed benchmarks reports the failure and its reason, rather than the benign
// "No benchmarks parsed." that hid command-not-found errors.
func TestTerminalSurfacesFailedRun(t *testing.T) {
	r := &archbench.RunResult{
		Target:   "remote",
		Mode:     archbench.ModeBench,
		Metadata: archbench.Metadata{OS: "linux", Arch: "amd64"},
		Runs: []archbench.ScenarioResult{{
			Name:     "small-sum",
			Command:  "go test ./... -bench=.",
			ExitCode: 127,
			Stderr:   "sh: 1: go: command not found",
		}},
	}

	cmd, out := captureCommand()
	Terminal(cmd, r)

	got := out.String()
	for _, want := range []string{"command failed (exit 127)", "go: command not found", "hint:", "Set PATH"} {
		if !strings.Contains(got, want) {
			t.Errorf("terminal output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "No benchmarks parsed") {
		t.Errorf("failed run should not report 'No benchmarks parsed':\n%s", got)
	}
}

func captureCommand() (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	return cmd, &out
}

func benchResult(target string, ns float64) *archbench.RunResult {
	return &archbench.RunResult{
		Target: target,
		Mode:   archbench.ModeBench,
		Metadata: archbench.Metadata{
			Arch: target,
		},
		Runs: []archbench.ScenarioResult{{
			Name:    "stream",
			Command: "go test ./stream -bench=.",
			Benchmarks: []archbench.Benchmark{{
				Name: "BenchmarkRead",
				Metrics: map[string]float64{
					archbench.MetricNsPerOp: ns,
				},
			}},
		}},
	}
}

func testResult(target string, status archbench.TestStatus) *archbench.RunResult {
	return &archbench.RunResult{
		Target: target,
		Mode:   archbench.ModeTest,
		Metadata: archbench.Metadata{
			Arch: target,
		},
		Runs: []archbench.ScenarioResult{{
			Name:    "race",
			Command: "go test ./... -json",
			Tests: []archbench.Test{{
				Name:   "TestRead",
				Status: status,
			}},
		}},
	}
}
