// Package report renders results to the terminal and markdown, and diffs two
// results.
package engine

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirrobot01/archbench"
	"github.com/sirrobot01/archbench/internal/ui"
)

// Printer is the subset of cobra.Command's output methods used by reports.
type Printer interface {
	Print(args ...any)
	Println(args ...any)
	Printf(format string, args ...any)
}

// Terminal writes a human-readable summary of a run.
func Terminal(p Printer, r *archbench.RunResult) {
	platform := "unknown"
	switch {
	case r.Metadata.OS != "" && r.Metadata.Arch != "":
		platform = r.Metadata.OS + "/" + r.Metadata.Arch
	case r.Metadata.OS != "":
		platform = r.Metadata.OS
	case r.Metadata.Arch != "":
		platform = r.Metadata.Arch
	}

	duration := "n/a"
	if r.DurationSeconds > 0 && r.DurationSeconds < 1 {
		duration = fmt.Sprintf("%.0fms", r.DurationSeconds*1000)
	} else if r.DurationSeconds >= 1 {
		duration = fmt.Sprintf("%.2fs", r.DurationSeconds)
	}

	p.Print(ui.Title(r.Target))
	p.Print(" ")
	p.Print(ui.Subtle("(" + platform + ")"))
	p.Printf(" mode=%s runs=%d duration=%s\n", r.Mode, len(r.Runs), duration)

	if r.Metadata.CPU != "" {
		p.Println(ui.Subtle(r.Metadata.CPU))
	}

	switch r.Mode {
	case archbench.ModeBench:
		terminalBench(p, r.Runs)
	case archbench.ModeTest:
		terminalTest(p, r.Runs)
	}
	p.Println()
}

func terminalBench(p Printer, runs []archbench.ScenarioResult) {
	for _, run := range runs {
		p.Printf("%s %s\n", ui.Subtle("run"), ui.Title(run.Name))
		rows := make([][]string, 0, len(run.Benchmarks))
		for _, b := range run.Benchmarks {
			rows = append(rows, []string{
				b.Name,
				fmt.Sprintf("%.0f", b.Metrics[archbench.MetricNsPerOp]),
				fmt.Sprintf("%.0f", b.Metrics[archbench.MetricBytesPerOp]),
				fmt.Sprintf("%.0f", b.Metrics[archbench.MetricAllocsPerOp]),
			})
		}
		if len(rows) == 0 {
			if failed(p, run) {
				continue
			}
			p.Println(ui.Subtle("No benchmarks parsed."))
			continue
		}
		p.Println(ui.RenderTable(
			[]string{"Benchmark", "ns/op", "B/op", "allocs/op"},
			rows,
			map[int]bool{1: true, 2: true, 3: true},
		))
	}
}

// exitCommandNotFound is the shell convention for a command that was not found
// on PATH -- the usual symptom of a toolchain missing from a non-interactive
// SSH or Docker shell.
const exitCommandNotFound = 127

// failed reports a non-zero run as an error line (with a stderr snippet when
// available) and returns whether it did so. A zero exit returns false, leaving
// rendering to the caller.
func failed(p Printer, run archbench.ScenarioResult) bool {
	if run.ExitCode == 0 {
		return false
	}
	p.Println(ui.Danger(fmt.Sprintf("command failed (exit %d)", run.ExitCode)))
	if run.Stderr != "" {
		p.Println(ui.Subtle(snippet(run.Stderr, 5)))
	}
	if run.ExitCode == exitCommandNotFound {
		p.Println(ui.Subtle("hint: command not found — SSH and Docker targets run a " +
			"non-interactive shell that does not source your login profile, so a " +
			"toolchain installed outside the default PATH is invisible. Set PATH via " +
			"the target's env."))
	}
	return true
}

// snippet returns the last n lines of s for a compact failure summary.
func snippet(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func terminalTest(p Printer, runs []archbench.ScenarioResult) {
	for _, run := range runs {
		if len(run.Tests) == 0 && run.ExitCode != 0 {
			p.Printf("%s %s\n", ui.Subtle("run"), ui.Title(run.Name))
			failed(p, run)
			continue
		}

		var pass, fail, skip int
		rows := make([][]string, 0, len(run.Tests))
		for _, t := range run.Tests {
			switch t.Status {
			case archbench.StatusPass:
				pass++
			case archbench.StatusFail:
				fail++
			case archbench.StatusSkip:
				skip++
			}
			rows = append(rows, []string{
				t.Name,
				ui.Status(string(t.Status)),
				fmt.Sprintf("%.2fs", t.ElapsedSeconds),
			})
		}
		p.Printf("%s %s  %s  %s  %s\n",
			ui.Subtle("run"),
			ui.Title(run.Name),
			ui.Success(fmt.Sprintf("%d passed", pass)),
			ui.Danger(fmt.Sprintf("%d failed", fail)),
			ui.Warning(fmt.Sprintf("%d skipped", skip)),
		)
		if len(rows) > 0 {
			p.Println(ui.RenderTable(
				[]string{"Test", "Status", "Elapsed"},
				rows,
				map[int]bool{2: true},
			))
		}
	}
}

// Markdown writes a run as a markdown section for PR comments or CI summaries.
func Markdown(p Printer, r *archbench.RunResult) {
	p.Printf("### %s `%s/%s`\n\n", r.Target, r.Metadata.OS, r.Metadata.Arch)
	switch r.Mode {
	case archbench.ModeBench:
		for _, run := range r.Runs {
			p.Printf("#### %s\n\n", run.Name)
			p.Println("| Benchmark | ns/op | B/op | allocs/op |")
			p.Println("|---|--:|--:|--:|")
			for _, b := range run.Benchmarks {
				p.Printf("| %s | %.0f | %.0f | %.0f |\n", b.Name,
					b.Metrics[archbench.MetricNsPerOp],
					b.Metrics[archbench.MetricBytesPerOp],
					b.Metrics[archbench.MetricAllocsPerOp])
			}
			p.Println()
		}
	case archbench.ModeTest:
		for _, run := range r.Runs {
			p.Printf("#### %s\n\n", run.Name)
			p.Println("| Test | Status | Elapsed (s) |")
			p.Println("|---|---|--:|")
			for _, t := range run.Tests {
				p.Printf("| %s | %s | %.2f |\n", t.Name, t.Status, t.ElapsedSeconds)
			}
			p.Println()
		}
	}
	p.Println()
}

// Compare writes a diff of two runs, with a as the baseline.
func Compare(p Printer, a, b *archbench.RunResult) error {
	if a.Mode != b.Mode {
		return errors.New("cannot compare different modes: " + string(a.Mode) + " vs " + string(b.Mode))
	}
	p.Print(ui.Title("Comparison"))
	p.Printf(" %s\n\n", ui.Subtle(a.Target+" ("+a.Metadata.Arch+") vs "+b.Target+" ("+b.Metadata.Arch+")"))

	switch a.Mode {
	case archbench.ModeBench:
		compareBench(p, a, b)
	case archbench.ModeTest:
		compareTest(p, a, b)
	}
	return nil
}

func compareBench(p Printer, a, b *archbench.RunResult) {
	candidateRuns := make(map[string]archbench.ScenarioResult, len(b.Runs))
	for _, run := range b.Runs {
		candidateRuns[run.Name] = run
	}

	seenRuns := make(map[string]bool, len(a.Runs)+len(b.Runs))
	for _, base := range a.Runs {
		seenRuns[base.Name] = true
		candidate, ok := candidateRuns[base.Name]
		p.Printf("%s %s\n", ui.Subtle("run"), ui.Title(base.Name))
		if !ok {
			rows := make([][]string, 0, len(base.Benchmarks))
			for _, ob := range base.Benchmarks {
				rows = append(rows, []string{ob.Name, fmt.Sprintf("%.0f", ob.Metrics[archbench.MetricNsPerOp]), ui.Subtle("-"), ui.Diff("(removed run)")})
			}
			p.Println(ui.RenderTable(
				[]string{"Benchmark", a.Target + " ns/op", b.Target + " ns/op", "Diff"},
				rows,
				map[int]bool{1: true, 2: true},
			))
			continue
		}

		candidateBenchmarks := make(map[string]archbench.Benchmark, len(candidate.Benchmarks))
		for _, nb := range candidate.Benchmarks {
			candidateBenchmarks[nb.Name] = nb
		}

		rows := make([][]string, 0, len(base.Benchmarks)+len(candidate.Benchmarks))
		seenBenchmarks := make(map[string]bool, len(base.Benchmarks)+len(candidate.Benchmarks))
		for _, ob := range base.Benchmarks {
			seenBenchmarks[ob.Name] = true
			nb, ok := candidateBenchmarks[ob.Name]
			if !ok {
				rows = append(rows, []string{ob.Name, fmt.Sprintf("%.0f", ob.Metrics[archbench.MetricNsPerOp]), ui.Subtle("-"), ui.Diff("(removed)")})
				continue
			}
			an := ob.Metrics[archbench.MetricNsPerOp]
			bn := nb.Metrics[archbench.MetricNsPerOp]
			rows = append(rows, []string{nb.Name, fmt.Sprintf("%.0f", an), fmt.Sprintf("%.0f", bn), ui.Diff(ratio(an, bn))})
		}
		for _, nb := range candidate.Benchmarks {
			if seenBenchmarks[nb.Name] {
				continue
			}
			rows = append(rows, []string{nb.Name, ui.Subtle("-"), fmt.Sprintf("%.0f", nb.Metrics[archbench.MetricNsPerOp]), ui.Diff("(new)")})
		}
		p.Println(ui.RenderTable(
			[]string{"Benchmark", a.Target + " ns/op", b.Target + " ns/op", "Diff"},
			rows,
			map[int]bool{1: true, 2: true},
		))
	}

	for _, candidate := range b.Runs {
		if seenRuns[candidate.Name] {
			continue
		}
		p.Printf("%s %s\n", ui.Subtle("run"), ui.Title(candidate.Name))
		rows := make([][]string, 0, len(candidate.Benchmarks))
		for _, nb := range candidate.Benchmarks {
			rows = append(rows, []string{nb.Name, ui.Subtle("-"), fmt.Sprintf("%.0f", nb.Metrics[archbench.MetricNsPerOp]), ui.Diff("(new run)")})
		}
		p.Println(ui.RenderTable(
			[]string{"Benchmark", a.Target + " ns/op", b.Target + " ns/op", "Diff"},
			rows,
			map[int]bool{1: true, 2: true},
		))
	}
}

// Regression is a benchmark whose ns/op grew from baseline to candidate by more
// than the caller's threshold.
type Regression struct {
	Run       string
	Benchmark string
	Baseline  float64 // baseline ns/op
	Candidate float64 // candidate ns/op
	Percent   float64 // signed percent change, always > threshold here
}

// BenchRegressions returns the benchmarks whose ns/op regressed from a
// (baseline) to b (candidate) by more than thresholdPct percent, matching runs
// and benchmarks by name. Benchmarks present on only one side are skipped: a
// missing data point is a new or removed benchmark, not a regression. It returns
// nil unless both results are bench mode.
func BenchRegressions(a, b *archbench.RunResult, thresholdPct float64) []Regression {
	if a.Mode != archbench.ModeBench || b.Mode != archbench.ModeBench {
		return nil
	}
	candidateRuns := make(map[string]archbench.ScenarioResult, len(b.Runs))
	for _, run := range b.Runs {
		candidateRuns[run.Name] = run
	}

	var regs []Regression
	for _, base := range a.Runs {
		candidate, ok := candidateRuns[base.Name]
		if !ok {
			continue
		}
		candidateBenchmarks := make(map[string]archbench.Benchmark, len(candidate.Benchmarks))
		for _, nb := range candidate.Benchmarks {
			candidateBenchmarks[nb.Name] = nb
		}
		for _, ob := range base.Benchmarks {
			nb, ok := candidateBenchmarks[ob.Name]
			if !ok {
				continue
			}
			an := ob.Metrics[archbench.MetricNsPerOp]
			bn := nb.Metrics[archbench.MetricNsPerOp]
			if an <= 0 {
				continue
			}
			pct := (bn - an) / an * 100
			if pct > thresholdPct {
				regs = append(regs, Regression{
					Run:       base.Name,
					Benchmark: ob.Name,
					Baseline:  an,
					Candidate: bn,
					Percent:   pct,
				})
			}
		}
	}
	return regs
}

// ratio describes b relative to a as a signed percentage.
func ratio(a, b float64) string {
	if a == 0 {
		return "n/a"
	}
	pct := (b - a) / a * 100
	switch {
	case pct > 0:
		return fmt.Sprintf("+%.1f%% slower", pct)
	case pct < 0:
		return fmt.Sprintf("%.1f%% faster", pct)
	default:
		return "0%"
	}
}

func compareTest(p Printer, a, b *archbench.RunResult) {
	candidateRuns := make(map[string]archbench.ScenarioResult, len(b.Runs))
	for _, run := range b.Runs {
		candidateRuns[run.Name] = run
	}

	seenRuns := make(map[string]bool, len(a.Runs)+len(b.Runs))
	totalDiverged := 0
	for _, baseRun := range a.Runs {
		seenRuns[baseRun.Name] = true
		candidate, ok := candidateRuns[baseRun.Name]
		candidateTests := make(map[string]archbench.Test, len(candidate.Tests))
		for _, candidateTest := range candidate.Tests {
			candidateTests[candidateTest.Name] = candidateTest
		}

		var diverged int
		rows := make([][]string, 0, len(baseRun.Tests)+len(candidate.Tests))
		seenTests := make(map[string]bool, len(baseRun.Tests)+len(candidate.Tests))

		p.Printf("%s %s\n", ui.Subtle("run"), ui.Title(baseRun.Name))
		for _, baseTest := range baseRun.Tests {
			seenTests[baseTest.Name] = true
			candidateTest, testOK := candidateTests[baseTest.Name]
			baseStatus := string(baseTest.Status)
			candidateStatus := "absent"
			if testOK {
				candidateStatus = string(candidateTest.Status)
			}

			marker := ""
			switch {
			case !ok:
				marker = "(removed run)"
				diverged++
			case baseStatus != candidateStatus:
				marker = "DIVERGES"
				diverged++
			}
			rows = append(rows, []string{baseTest.Name, ui.Status(baseStatus), ui.Status(candidateStatus), ui.Diff(marker)})
		}
		for _, candidateTest := range candidate.Tests {
			if seenTests[candidateTest.Name] {
				continue
			}
			diverged++
			rows = append(rows, []string{candidateTest.Name, ui.Status("absent"), ui.Status(string(candidateTest.Status)), ui.Diff("DIVERGES")})
		}
		totalDiverged += diverged
		p.Println(ui.RenderTable(
			[]string{"Test", a.Target, b.Target, "Result"},
			rows,
			nil,
		))
	}
	for _, candidate := range b.Runs {
		if seenRuns[candidate.Name] {
			continue
		}
		p.Printf("%s %s\n", ui.Subtle("run"), ui.Title(candidate.Name))
		rows := make([][]string, 0, len(candidate.Tests))
		for _, candidateTest := range candidate.Tests {
			totalDiverged++
			rows = append(rows, []string{candidateTest.Name, ui.Status("absent"), ui.Status(string(candidateTest.Status)), ui.Diff("(new run)")})
		}
		p.Println(ui.RenderTable(
			[]string{"Test", a.Target, b.Target, "Result"},
			rows,
			nil,
		))
	}
	p.Printf("\n%s\n", ui.Subtle(fmt.Sprintf("%d test(s) diverge across targets", totalDiverged)))
}
