In order to run the benchmarks one needs to do the following:

Build the test file:

```
go test -c
```

Run the test file to generate the profile.
```
./validate.test -test-run=none -test.bench=MakeMulti -test.cpuprofile cpu.out
```

To run the pprof tool:
```
go tool pprof validate.test cpu.out
```

In the pprof tool run web list makeValid
to get the profile of the `makeValid` function.

