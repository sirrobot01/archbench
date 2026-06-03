# Getting Started

ArchBench runs a suite from `archbench.yaml`. A suite declares targets, one or
more named run groups, and the parser used to normalize output.

## Local Benchmarks

```yaml
name: local-demo
mode: bench

targets:
  - name: local
    type: local

runs:
  - name: all
    command: go test ./... -run '^$' -bench=. -benchmem -count=5

parser: go-test
```

Run it:

```sh
archbench run --target local
```

Results are written to `archbench-results/<target>.json`. Each target artifact
contains a `runs` array with one entry per configured run group.

## Multiple Benchmark Groups

Use `runs:` when different packages, files, or benchmark families need distinct
commands, setup, counts, or environment.

```yaml
runs:
  - name: parser
    command: go test ./internal/parser/... -run '^$' -bench=. -benchmem -count=5

  - name: stream
    env:
      GOMAXPROCS: "4"
    command: go test ./pkg/stream/... -run '^$' -bench='BenchmarkRead|BenchmarkWrite' -benchmem -count=10
```

## Target Concurrency

By default, ArchBench runs targets sequentially. To run multiple targets at the
same time, set a concurrency limit:

```sh
archbench run --concurrency 2
```

Concurrency is target-level: each selected target executes its configured
`runs` sequentially, while multiple targets may run at the same time. Result
summaries are printed in spec order after the selected targets complete.

## SSH Benchmarks

```yaml
name: remote-demo
mode: bench

targets:
  - name: amd64-box
    type: ssh
    host: bench-box

runs:
  - name: all
    setup:
      - go mod download
    command: go test ./... -run '^$' -bench=. -benchmem -count=10

parser: go-test
```

`host` can be a `~/.ssh/config` alias. ArchBench packages the project, uploads
it to a temporary remote work directory, runs setup and the command there, then
removes the work directory.

## Cache Behavior

Runners expose a cache directory through `$ARCHBENCH_CACHE`. For `go-test`,
ArchBench injects:

```yaml
GOCACHE: $ARCHBENCH_CACHE/go-build
GOMODCACHE: $ARCHBENCH_CACHE/go-mod
```

Use `--no-cache` to force an ephemeral cold run.

## Test Mode

```yaml
mode: test
runs:
  - name: race
    command: go test ./... -race -json
parser: go-test
```

In test mode, result artifacts contain test statuses instead of benchmark
metrics, and comparisons report cross-target status divergence.
