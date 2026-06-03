package archbench

import "time"

// RunResult is the outcome of executing a suite on one target.
type RunResult struct {
	Target          string    `json:"target"`
	Mode            Mode      `json:"mode"`
	Metadata        Metadata  `json:"metadata"`
	Started         time.Time `json:"started"`
	DurationSeconds float64   `json:"duration_seconds"`

	Runs []ScenarioResult `json:"runs"`

	Error string `json:"error,omitempty"`
}

// ScenarioResult is the parsed output of one named spec run on a target.
type ScenarioResult struct {
	Name            string    `json:"name"`
	Command         string    `json:"command"`
	Started         time.Time `json:"started"`
	DurationSeconds float64   `json:"duration_seconds"`
	ExitCode        int       `json:"exit_code,omitempty"`

	// Stderr is captured only for a failed run, so the reason a command exited
	// non-zero (e.g. "go: command not found") survives into the result.
	Stderr string `json:"stderr,omitempty"`

	Benchmarks []Benchmark `json:"benchmarks,omitempty"`
	Tests      []Test      `json:"tests,omitempty"`

	Error string `json:"error,omitempty"`
}

// Metadata describes the environment a run executed in.
type Metadata struct {
	Arch      string            `json:"arch"`
	OS        string            `json:"os"`
	Kernel    string            `json:"kernel,omitempty"`
	CPU       string            `json:"cpu,omitempty"`
	Toolchain map[string]string `json:"toolchain,omitempty"`

	// Runner optionally names the execution environment, e.g. the GitHub
	// Actions runner label. Omitted for local and most remote runners.
	Runner string `json:"runner,omitempty"`
}

// Well-known benchmark metric keys. Parsers may report others.
const (
	MetricNsPerOp     = "ns_per_op"
	MetricAllocsPerOp = "allocs_per_op"
	MetricBytesPerOp  = "bytes_per_op"
	MetricMBPerSec    = "mb_per_sec"
)

// Benchmark is one parsed benchmark, aggregated across samples.
type Benchmark struct {
	Name       string             `json:"name"`
	Iterations int                `json:"iterations"`
	Runs       int                `json:"runs,omitempty"`
	Metrics    map[string]float64 `json:"metrics"`
}

// TestStatus is the outcome of a single test.
type TestStatus string

const (
	StatusPass TestStatus = "pass"
	StatusFail TestStatus = "fail"
	StatusSkip TestStatus = "skip"
)

// Test is one parsed test result.
type Test struct {
	Name           string     `json:"name"`
	Status         TestStatus `json:"status"`
	ElapsedSeconds float64    `json:"elapsed_seconds"`
	Output         string     `json:"output,omitempty"`
}
