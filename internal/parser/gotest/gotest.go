// Package gotest parses Go benchmark and test output.
package gotest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirrobot01/archbench/spec"
)

type parser struct{}

// New returns a parser for Go benchmark output (go test -bench) and test
// output (go test -json).
func New() spec.Parser { return parser{} }

func (parser) Name() string { return "go-test" }

func (parser) Modes() []spec.Mode {
	return []spec.Mode{spec.ModeBench, spec.ModeTest}
}

func (p parser) Parse(mode spec.Mode, out *spec.Output) (*spec.Parsed, error) {
	switch mode {
	case spec.ModeBench:
		return p.parseBench(out)
	case spec.ModeTest:
		return p.parseTest(out)
	default:
		return nil, fmt.Errorf("go-test: unsupported mode %q", mode)
	}
}

func (parser) parseBench(out *spec.Output) (*spec.Parsed, error) {
	type agg struct {
		iterations int
		sums       map[string]float64
		n          int
	}
	byName := map[string]*agg{}
	var order []string
	pkg := ""

	sc := scanner(out.Stdout)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "pkg: ") {
			pkg = strings.TrimSpace(line[len("pkg: "):])
			continue
		}

		// A benchmark line is "BenchmarkName-8 <iters> <val> <unit> ...": the
		// name, the iteration count, then value/unit metric pairs. Splitting on
		// fields avoids a regex match and a throwaway metrics map per line.
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		iters, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		if !hasMetric(fields[2:]) {
			continue
		}

		name := scopedName(pkg, trimProcSuffix(fields[0]))
		a := byName[name]
		if a == nil {
			a = &agg{sums: map[string]float64{}}
			byName[name] = a
			order = append(order, name)
		}
		a.n++
		a.iterations = iters
		addMetrics(fields[2:], a.sums)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("go-test: scan: %w", err)
	}

	benches := make([]spec.Benchmark, 0, len(order))
	for _, name := range order {
		a := byName[name]
		metrics := make(map[string]float64, len(a.sums))
		for k, sum := range a.sums {
			metrics[k] = sum / float64(a.n)
		}
		benches = append(benches, spec.Benchmark{
			Name:       name,
			Iterations: a.iterations,
			Runs:       a.n,
			Metrics:    metrics,
		})
	}
	return &spec.Parsed{Benchmarks: benches}, nil
}

// hasMetric reports whether fields holds at least one value/unit metric pair,
// so a benchmark line with no parseable metrics is skipped before it records an
// empty benchmark.
func hasMetric(fields []string) bool {
	for i := 0; i+1 < len(fields); i += 2 {
		if _, err := strconv.ParseFloat(fields[i], 64); err == nil {
			return true
		}
	}
	return false
}

// addMetrics adds value/unit pairs like "1200 ns/op 512 B/op" into sums.
func addMetrics(fields []string, sums map[string]float64) {
	for i := 0; i+1 < len(fields); i += 2 {
		val, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			continue
		}
		sums[metricKey(fields[i+1])] += val
	}
}

// metricKey maps a go test unit to its well-known metric key, passing through
// custom b.ReportMetric units unchanged.
func metricKey(unit string) string {
	switch unit {
	case "ns/op":
		return spec.MetricNsPerOp
	case "B/op":
		return spec.MetricBytesPerOp
	case "allocs/op":
		return spec.MetricAllocsPerOp
	case "MB/s":
		return spec.MetricMBPerSec
	default:
		return unit
	}
}

// trimProcSuffix strips the "-8" GOMAXPROCS suffix go test appends to a
// benchmark name, leaving names without one untouched.
func trimProcSuffix(name string) string {
	i := strings.LastIndexByte(name, '-')
	if i < 0 || i+1 == len(name) {
		return name
	}
	for _, r := range name[i+1:] {
		if r < '0' || r > '9' {
			return name
		}
	}
	return name[:i]
}

// testEvent is a subset of a `go test -json` event.
type testEvent struct {
	Package string
	Action  string
	Test    string
	Elapsed float64
	Output  string
}

func (parser) parseTest(out *spec.Output) (*spec.Parsed, error) {
	type acc struct {
		status  spec.TestStatus
		elapsed float64
		output  strings.Builder
	}
	byName := map[string]*acc{}
	var order []string

	sc := scanner(out.Stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] != '{' {
			continue
		}
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) != nil || ev.Test == "" {
			continue
		}
		name := scopedName(ev.Package, ev.Test)
		a := byName[name]
		if a == nil {
			a = &acc{status: spec.StatusFail}
			byName[name] = a
			order = append(order, name)
		}
		switch ev.Action {
		case "pass":
			a.status, a.elapsed = spec.StatusPass, ev.Elapsed
		case "fail":
			a.status, a.elapsed = spec.StatusFail, ev.Elapsed
		case "skip":
			a.status, a.elapsed = spec.StatusSkip, ev.Elapsed
		case "output":
			a.output.WriteString(ev.Output)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("go-test: scan: %w", err)
	}

	tests := make([]spec.Test, 0, len(order))
	for _, name := range order {
		a := byName[name]
		t := spec.Test{Name: name, Status: a.status, ElapsedSeconds: a.elapsed}
		if a.status != spec.StatusPass {
			t.Output = strings.TrimSpace(a.output.String())
		}
		tests = append(tests, t)
	}
	return &spec.Parsed{Tests: tests}, nil
}

func scopedName(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func scanner(s string) *bufio.Scanner {
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)
	return sc
}
