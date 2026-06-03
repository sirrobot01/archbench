<p align="center">
  <img src="docs/gopher.png" alt="ArchBench gopher mascot" width="360">
</p>

# ArchBench

ArchBench is a lightweight benchmark and test orchestration tool for native
cross-architecture workflows. It is aimed at developers who work on ARM64
laptops but need reproducible results from AMD64 Linux machines, homelab boxes,
or other SSH-accessible benchmark hosts.

- local, SSH, Docker, and GitHub Actions targets
- `bench` and `test` modes
- Go output normalization through the built-in `go-test` parser
- JSON result artifacts
- terminal and Markdown reports
- local and remote build-cache wiring through `$ARCHBENCH_CACHE`
- GitHub Actions workflow generation with `archbench generate`

## Install

### Homebrew (macOS)

```sh
brew install --cask sirrobot01/tap/archbench
```

ArchBench ships as a Homebrew cask, which is macOS-only. On Linux, use Go or a
prebuilt binary below.

### Go

```sh
go install github.com/sirrobot01/archbench/cmd/archbench@latest
```

### Prebuilt binaries

Download a `tar.gz` for your OS/arch from the
[releases page](https://github.com/sirrobot01/archbench/releases).

### From source

```sh
go build ./cmd/archbench
./archbench version
```

**Note**: Windows is intentionally excluded
until the runner layer supports non-POSIX process handling.

## Quick Start

Create a spec:

```sh
archbench init
```

Run all configured targets:

```sh
archbench run
```

Run one target:

```sh
archbench run --target local
```

Run multiple targets concurrently:

```sh
archbench run --concurrency 2
```

The default concurrency is `1`, so runs remain sequential unless you opt in.

Render saved results:

```sh
archbench report --format md
```

Compare two result artifacts:

```sh
archbench compare baseline.json candidate.json
```

## Example

This repository includes a small benchmark fixture:

```sh
archbench run \
  --spec examples/go-basic/archbench.yaml \
  --dir examples/go-basic \
  --target local \
  --no-cache
```

## Spec Shape

```yaml
name: my-suite

mode: bench

targets:
  - name: local
    type: local

  - name: amd64-box
    type: ssh
    host: bench-box
    setup:
      - go mod download     # provisioned once, before any run

  - name: amd64-container
    type: docker
    image: golang:1.26
    platform: linux/amd64   # optional; pins a non-native arch via emulation

  - name: ci
    type: github-actions
    runsOn: ubuntu-latest   # runner label for the generated workflow

runs:
  - name: parser
    setup:
      - go generate ./internal/parser/...   # per-run preparation
    command: go test ./internal/parser/... -run '^$' -bench=. -benchmem -count=10

  - name: stream
    command: go test ./pkg/stream/... -run '^$' -bench='BenchmarkRead|BenchmarkWrite' -benchmem -count=10

parser: go-test
```

Each selected target executes every entry in `runs` in order. The saved result
artifact contains one top-level target result with a `runs` array, so reports and
comparisons can keep benchmark groups separate.

A target's `setup` runs once, after the project is uploaded but before any run —
use it to provision the environment (install system packages, warm the module
cache). A run's `setup` runs before that single run's command, for per-run
preparation. Both share the build cache wired through `$ARCHBENCH_CACHE`, so a
`go mod download` in a target's `setup` warms the same module cache the runs use.

A target's `env` applies to its setup and to every run on it; a run's own `env`
overrides it. Values may reference `$ARCHBENCH_CACHE` and, like run env, may hold
secrets — they are written to a private file on the target, never passed on a
command line.

SSH hosts are delegated to the system `ssh` client. Host aliases, identities,
ProxyJump, multiplexing, agents, and known_hosts behavior come from the user's
existing SSH setup unless explicitly overridden in the spec.

## Docker Targets

A `docker` target runs the suite inside a container built from `image`. ArchBench
creates one container per target, syncs the project into it, runs each group with
`docker exec`, and removes the container afterward. Pin a non-native `platform`
(e.g. `linux/amd64`) to exercise another architecture through the daemon's
emulation — ArchBench flags such runs as untrustworthy for benchmark timings.

```sh
archbench run --target amd64-container
```

## GitHub Actions

A `github-actions` target executes on the current machine, so it runs natively on
a GitHub-hosted runner after the workflow checks out the project. Generate a
workflow that wires up those targets as a build matrix:

```sh
archbench generate
```

This writes `.github/workflows/archbench.yml` with one matrix job per
`github-actions` target, each running `archbench run` on its configured `runsOn`
runner and uploading the result artifact.

## Project Status

ArchBench is still in beta. The core local/SSH path builds and has unit coverage, but
the public API and result schema should still be treated as changeable.
