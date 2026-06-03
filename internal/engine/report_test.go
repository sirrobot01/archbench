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
