# go-basic

The smallest benchmark suite: a single `Sum` function with two benchmark sizes.
`BenchmarkSumSmall` and `BenchmarkSumLarge` report `ns/op` (no allocations), a
clean starting point for a first cross-architecture run.

```sh
archbench run --spec examples/go-basic/archbench.yaml --dir examples/go-basic --target local --no-cache
```
