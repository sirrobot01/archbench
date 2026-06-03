<p align="center">
  <img src="docs/gopher.png" alt="ArchBench gopher mascot" width="360">
</p>

# ArchBench

ArchBench is a lightweight benchmark and test orchestration tool for native
cross-architecture workflows. It is aimed at developers who work on ARM64
laptops but need reproducible results from AMD64 Linux machines, homelab boxes,
or other SSH-accessible benchmark hosts.

v0.1 focuses on the core workflow:

- local and SSH targets
- `bench` and `test` modes
- Go output normalization through the built-in `go-test` parser
- JSON result artifacts
- terminal and Markdown reports
- local and remote build-cache wiring through `$ARCHBENCH_CACHE`

Docker and GitHub Actions support are planned after the first release.

## Install From Source

```sh
go install github.com/sirrobot01/archbench/cmd/archbench@latest
```

For local development:

```sh
go build ./cmd/archbench
./archbench version
```

## Releases

GitHub releases are produced by GoReleaser when a version tag is pushed:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Release binaries embed the GoReleaser version, so `archbench version` prints the
tag-derived version in packaged builds.

v0.1 release archives target Linux and macOS. Windows is intentionally excluded
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

runs:
  - name: parser
    setup:
      - go mod download
    command: go test ./internal/parser/... -run '^$' -bench=. -benchmem -count=10

  - name: stream
    command: go test ./pkg/stream/... -run '^$' -bench='BenchmarkRead|BenchmarkWrite' -benchmem -count=10

parser: go-test
```

Each selected target executes every entry in `runs` in order. The saved result
artifact contains one top-level target result with a `runs` array, so reports and
comparisons can keep benchmark groups separate.

SSH hosts are delegated to the system `ssh` client. Host aliases, identities,
ProxyJump, multiplexing, agents, and known_hosts behavior come from the user's
existing SSH setup unless explicitly overridden in the spec.

## Project Status

ArchBench is pre-v0.1. The core local/SSH path builds and has unit coverage, but
the public API and result schema should still be treated as changeable.
