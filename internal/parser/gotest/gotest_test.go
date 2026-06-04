package gotest

import (
	"testing"

	"github.com/sirrobot01/archbench/spec"
)

func TestParseBenchAggregatesAndScopesByPackage(t *testing.T) {
	p := New()
	parsed, err := p.Parse(spec.ModeBench, &spec.Output{Stdout: `
goos: darwin
goarch: arm64
pkg: github.com/acme/demo/a
BenchmarkRead-8 100 10 ns/op 64 B/op 1 allocs/op 12 MB/s
BenchmarkRead-8 100 14 ns/op 96 B/op 3 allocs/op 16 MB/s
pkg: github.com/acme/demo/b
BenchmarkRead-8 50 30 ns/op 128 B/op 4 allocs/op
`})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(parsed.Benchmarks) != 2 {
		t.Fatalf("benchmarks = %d, want 2: %#v", len(parsed.Benchmarks), parsed.Benchmarks)
	}

	a := parsed.Benchmarks[0]
	if a.Name != "github.com/acme/demo/a.BenchmarkRead" {
		t.Fatalf("first benchmark name = %q", a.Name)
	}
	if a.Runs != 2 {
		t.Errorf("runs = %d, want 2", a.Runs)
	}
	if got := a.Metrics[spec.MetricNsPerOp]; got != 12 {
		t.Errorf("ns/op = %v, want 12", got)
	}
	if got := a.Metrics[spec.MetricBytesPerOp]; got != 80 {
		t.Errorf("B/op = %v, want 80", got)
	}
	if got := a.Metrics[spec.MetricAllocsPerOp]; got != 2 {
		t.Errorf("allocs/op = %v, want 2", got)
	}
	if got := a.Metrics[spec.MetricMBPerSec]; got != 14 {
		t.Errorf("MB/s = %v, want 14", got)
	}

	if parsed.Benchmarks[1].Name != "github.com/acme/demo/b.BenchmarkRead" {
		t.Errorf("second benchmark name = %q", parsed.Benchmarks[1].Name)
	}
}

func TestParseTestScopesByPackageAndCapturesFailures(t *testing.T) {
	p := New()
	parsed, err := p.Parse(spec.ModeTest, &spec.Output{Stdout: `
{"Package":"github.com/acme/demo/a","Action":"run","Test":"TestRead"}
{"Package":"github.com/acme/demo/a","Action":"pass","Test":"TestRead","Elapsed":0.03}
{"Package":"github.com/acme/demo/b","Action":"run","Test":"TestRead"}
{"Package":"github.com/acme/demo/b","Action":"output","Test":"TestRead","Output":"race detected\n"}
{"Package":"github.com/acme/demo/b","Action":"fail","Test":"TestRead","Elapsed":0.2}
{"Package":"github.com/acme/demo/b","Action":"skip","Test":"TestSkipped","Elapsed":0}
`})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(parsed.Tests) != 3 {
		t.Fatalf("tests = %d, want 3: %#v", len(parsed.Tests), parsed.Tests)
	}

	if parsed.Tests[0].Name != "github.com/acme/demo/a.TestRead" || parsed.Tests[0].Status != spec.StatusPass {
		t.Errorf("first test = %#v", parsed.Tests[0])
	}
	if parsed.Tests[1].Name != "github.com/acme/demo/b.TestRead" || parsed.Tests[1].Status != spec.StatusFail {
		t.Errorf("second test = %#v", parsed.Tests[1])
	}
	if parsed.Tests[1].Output != "race detected" {
		t.Errorf("failure output = %q", parsed.Tests[1].Output)
	}
	if parsed.Tests[2].Status != spec.StatusSkip {
		t.Errorf("third test status = %q, want skip", parsed.Tests[2].Status)
	}
}
