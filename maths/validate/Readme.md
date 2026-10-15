# Geometry validation

Run this package's tests from the repository root:

```sh
go test -mod=vendor ./maths/validate
```

The make-valid performance benchmarks live in `maths/makevalid`, not in this
package. For a CPU profile, run from the repository root:

```sh
go test -mod=vendor ./maths/makevalid -run '^$' -bench '^BenchmarkMakeValid5PolyA$' -cpuprofile cpu.out
go tool pprof cpu.out
```

See [the make-valid algorithm](../makevalid/README.md). Profiling writes local
artifacts; do not add them to source commits.
