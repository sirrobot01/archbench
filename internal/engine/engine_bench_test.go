package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sirrobot01/archbench/internal/parser/gotest"
	"github.com/sirrobot01/archbench/spec"
)

// stubRunner is an in-memory Runner that returns canned bench output for every
// Execute, so a benchmark can measure RunJob's orchestration and parsing
// without spawning a subprocess.
type stubRunner struct{ stdout string }

func (stubRunner) Prepare(context.Context) error                            { return nil }
func (stubRunner) Setup(context.Context, []string, map[string]string) error { return nil }
func (stubRunner) Cleanup(context.Context) error                            { return nil }
func (stubRunner) Capabilities() spec.Capabilities                          { return spec.Capabilities{Arch: "amd64"} }

func (r stubRunner) Execute(context.Context, spec.Run) (*spec.Output, error) {
	return &spec.Output{
		Stdout:    r.stdout,
		Arch:      "amd64",
		OS:        "linux",
		Toolchain: map[string]string{"go": "1.26.0"},
	}, nil
}

// canned bench output for one run: a handful of benchmarks at -count=10.
func cannedBench(benches, count int) string {
	var b strings.Builder
	b.WriteString("goos: linux\ngoarch: amd64\npkg: github.com/acme/demo\n")
	for n := 0; n < benches; n++ {
		for c := 0; c < count; c++ {
			fmt.Fprintf(&b, "BenchmarkScenario%d-8\t%d\t%d.0 ns/op\t%d B/op\t%d allocs/op\n", n, 1000+c, 120+c, 48+n, n%4)
		}
	}
	b.WriteString("PASS\n")
	return b.String()
}

func BenchmarkRunJob(b *testing.B) {
	r := stubRunner{stdout: cannedBench(10, 10)}
	p := gotest.New()

	runs := make([]spec.Run, 8)
	for i := range runs {
		runs[i] = spec.Run{Name: fmt.Sprintf("run%d", i), Command: "go test -bench=."}
	}
	job := spec.Job{
		ProtocolVersion: spec.ProtocolVersion,
		Mode:            spec.ModeBench,
		Parser:          "go-test",
		Env:             map[string]string{"GOCACHE": "$ARCHBENCH_CACHE/go-build"},
		Runs:            runs,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := RunJob(context.Background(), r, p, job)
		if err != nil {
			b.Fatal(err)
		}
		if len(res.Runs) != len(runs) {
			b.Fatalf("runs = %d, want %d", len(res.Runs), len(runs))
		}
	}
}
