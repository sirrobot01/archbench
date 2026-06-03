// Package gotest parses Go benchmark and test output.
package gotest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirrobot01/archbench"
)

type parser struct{}

// New returns a parser for Go benchmark output (go test -bench) and test
// output (go test -json).
func New() archbench.Parser { return parser{} }

func (parser) Name() string { return "go-test" }

func (parser) Modes() []archbench.Mode {
	return []archbench.Mode{archbench.ModeBench, archbench.ModeTest}
}

func (p parser) Parse(mode archbench.Mode, out *archbench.Output) (*archbench.Parsed, error) {
	switch mode {
	case archbench.ModeBench:
		return p.parseBench(out)
	case archbench.ModeTest:
		return p.parseTest(out)
	default:
		return nil, fmt.Errorf("go-test: unsupported mode %q", mode)
	}
}

// e.g. "BenchmarkReadStream-8   100000   1200 ns/op   512 B/op   2 allocs/op"
var benchLine = regexp.MustCompile(`^(Benchmark\S+?)(?:-\d+)?\s+(\d+)\s+(.*)$`)

func (parser) parseBench(out *archbench.Output) (*archbench.Parsed, error) {
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
			pkg = strings.TrimSpace(strings.TrimPrefix(line, "pkg: "))
			continue
		}

		m := benchLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		metrics := parseMetrics(m[3])
		if len(metrics) == 0 {
			continue
		}
		name := scopedName(pkg, m[1])
		a := byName[name]
		if a == nil {
			a = &agg{sums: map[string]float64{}}
			byName[name] = a
			order = append(order, name)
		}
		a.n++
		a.iterations, _ = strconv.Atoi(m[2])
		for k, v := range metrics {
			a.sums[k] += v
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("go-test: scan: %w", err)
	}

	benches := make([]archbench.Benchmark, 0, len(order))
	for _, name := range order {
		a := byName[name]
		metrics := make(map[string]float64, len(a.sums))
		for k, sum := range a.sums {
			metrics[k] = sum / float64(a.n)
		}
		benches = append(benches, archbench.Benchmark{
			Name:       name,
			Iterations: a.iterations,
			Runs:       a.n,
			Metrics:    metrics,
		})
	}
	return &archbench.Parsed{Benchmarks: benches}, nil
}

// parseMetrics reads pairs like "1200 ns/op 512 B/op" into metric keys.
func parseMetrics(s string) map[string]float64 {
	fields := strings.Fields(s)
	metrics := map[string]float64{}
	for i := 0; i+1 < len(fields); i += 2 {
		val, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "ns/op":
			metrics[archbench.MetricNsPerOp] = val
		case "B/op":
			metrics[archbench.MetricBytesPerOp] = val
		case "allocs/op":
			metrics[archbench.MetricAllocsPerOp] = val
		case "MB/s":
			metrics[archbench.MetricMBPerSec] = val
		default:
			metrics[fields[i+1]] = val // custom b.ReportMetric unit
		}
	}
	return metrics
}

// testEvent is a subset of a `go test -json` event.
type testEvent struct {
	Package string
	Action  string
	Test    string
	Elapsed float64
	Output  string
}

func (parser) parseTest(out *archbench.Output) (*archbench.Parsed, error) {
	type acc struct {
		status  archbench.TestStatus
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
			a = &acc{status: archbench.StatusFail}
			byName[name] = a
			order = append(order, name)
		}
		switch ev.Action {
		case "pass":
			a.status, a.elapsed = archbench.StatusPass, ev.Elapsed
		case "fail":
			a.status, a.elapsed = archbench.StatusFail, ev.Elapsed
		case "skip":
			a.status, a.elapsed = archbench.StatusSkip, ev.Elapsed
		case "output":
			a.output.WriteString(ev.Output)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("go-test: scan: %w", err)
	}

	tests := make([]archbench.Test, 0, len(order))
	for _, name := range order {
		a := byName[name]
		t := archbench.Test{Name: name, Status: a.status, ElapsedSeconds: a.elapsed}
		if a.status != archbench.StatusPass {
			t.Output = strings.TrimSpace(a.output.String())
		}
		tests = append(tests, t)
	}
	return &archbench.Parsed{Tests: tests}, nil
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
