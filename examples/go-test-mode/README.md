# go-test-mode

A `mode: test` suite. Instead of benchmark metrics, archbench records each
test's pass/fail/skip status, and `archbench compare` reports where status
diverges across targets.

The suite pins `go test ./... -json` so the `go-test` parser can read structured
events.

```sh
archbench run --spec examples/go-test-mode/archbench.yaml --dir examples/go-test-mode --target local
```

`TestArchSpecific` skips on every architecture except amd64. Run the suite on two
targets of different architectures and compare them to see the divergence test
mode is built to surface:

```sh
archbench compare archbench-results/arm64.json archbench-results/amd64.json
```
