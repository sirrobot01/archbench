# go-allocs

A benchmark suite where memory metrics matter. `Concat` joins strings with naive
`+=`; `Builder` uses a `strings.Builder`. Their `ns/op`, `B/op`, and `allocs/op`
differ sharply, so the `-benchmem` columns in reports and comparisons carry real
signal.

```sh
archbench run --spec examples/go-allocs/archbench.yaml --dir examples/go-allocs --target local --no-cache
```

Compare two saved runs (e.g. an ARM64 laptop vs an AMD64 host) to see how the
allocation-heavy path scales across architectures:

```sh
archbench compare archbench-results/local.json archbench-results/remote.json
```

## Running across machines

[`archbench.remote.yaml`](archbench.remote.yaml) runs the same suite on `local`,
an SSH host (in exec mode), and a Docker container. Fill in `host` (and adjust
`PATH`/`image`) for your setup, then run all targets:

```sh
archbench run --spec examples/go-allocs/archbench.remote.yaml --dir examples/go-allocs --concurrency 2
```

