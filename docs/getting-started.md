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

### Toolchain not on PATH

Commands run over a non-interactive SSH shell, which does not source the login
profile (`~/.profile`, `~/.bashrc`, `/etc/profile.d`). A toolchain installed
outside the default PATH — for example Go under `/usr/local/go/bin` — is then
invisible, and runs fail with `command failed (exit 127)`. Pin the PATH on the
target so the command can find it:

```yaml
targets:
  - name: amd64-box
    type: ssh
    host: bench-box
    env:
      # Values are literal, so list the directories rather than referencing
      # $PATH. Put the toolchain dir first.
      PATH: /usr/local/go/bin:/usr/local/bin:/usr/bin:/bin
```

The same applies to Docker targets whose image places a toolchain off the
default PATH.

## Docker Benchmarks

```yaml
name: container-demo
mode: bench

targets:
  - name: amd64
    type: docker
    image: golang:1.26
    platform: linux/amd64   # optional

runs:
  - name: all
    command: go test ./... -run '^$' -bench=. -benchmem -count=10

parser: go-test
```

ArchBench creates one container per target from `image`, syncs the project into
an isolated work directory, runs each group with `docker exec`, and force-removes
the container on cleanup. `platform` is optional; when it differs from the host
architecture the run is emulated and benchmark timings are reported as
untrustworthy. The same project-sync rules as SSH apply (Git-tracked plus
untracked, non-ignored files).

## GitHub Actions

A `github-actions` target runs on the current machine, which on CI is the
GitHub-hosted runner after `actions/checkout`.

```yaml
targets:
  - name: ci-amd64
    type: github-actions
    runsOn: ubuntu-latest

  - name: ci-arm64
    type: github-actions
    runsOn: ubuntu-24.04-arm
```

Generate a workflow that runs each `github-actions` target as a matrix job:

```sh
archbench generate
```

This writes `.github/workflows/archbench.yml`. Each job checks out the project,
installs ArchBench, runs `archbench run --target <name>` on its `runsOn` runner
(defaulting to `ubuntu-latest`), and uploads the JSON artifact. Results carry the
runner label in their metadata so reports record which CI machine produced them.

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
