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

Add `--threshold` to fail (non-zero exit) when any benchmark's `ns/op`
regresses past that percentage — useful as a CI gate:

```sh
archbench compare baseline.json candidate.json --threshold 50
```

## Examples

The [`examples/`](examples/) directory has self-contained suites you can run
locally — a basic benchmark, one with meaningful memory metrics, and a test-mode
suite. Each is its own module with its own spec:

```sh
archbench run \
  --spec examples/go-basic/archbench.yaml \
  --dir examples/go-basic \
  --target local \
  --no-cache
```

See [examples/README.md](examples/README.md) for the full list, including a
multi-target spec that adds SSH (exec mode) and Docker targets.

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

  - name: amd64-container
    type: docker
    image: golang:1.26
    platform: linux/amd64   # pin a non-native arch via emulation

  - name: ci
    type: github-actions
    runsOn: ubuntu-latest

runs:
  - name: parser
    command: go test ./internal/parser/... -run '^$' -bench=. -benchmem -count=10

  - name: stream
    command: go test ./pkg/stream/... -run '^$' -bench='BenchmarkRead|BenchmarkWrite' -benchmem -count=10

parser: go-test
```

Each selected target executes every entry in `runs` in order, writing one
`archbench-results/<target>.json` artifact with a `runs` array so reports and
comparisons keep benchmark groups separate. Per-target `setup`/`env`, exec mode,
Docker, GitHub Actions, caching, and test mode are covered in the docs below.

## Documentation

- [Getting Started](docs/getting-started.md) — full spec reference: local, SSH,
  Docker, and GitHub Actions targets, `setup`/`env`, exec mode, PATH setup,
  caching, and test mode.
- [Security Model](docs/security.md) — SSH and Docker trust, project sync, and
  how environment secrets are handled.

## Project Status

ArchBench is still in beta. The core local/SSH path builds and has unit coverage, but
the public API and result schema should still be treated as changeable.
