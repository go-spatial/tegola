# Fuzzing

Fuzzing the wkb package requires go-fuzz: https://github.com/dvyukov/go-fuzz

The first step should be to create the Fuzz program:

`
go-fuzz-build github.com/go-spatial/tegola/geom/encoding/wkb/internal/fuzz
`

This will create a binary called wkb-fuzz.zip in the current working directory.


To run the fuzzing function on the corpus:

`
go-fuzz -bin=/path/to/wkb-fuzz.zip -workdir=/path/to/tegola/encoding/wkb/internal/fuzz
`
