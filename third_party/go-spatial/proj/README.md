[![Build Status](https://travis-ci.org/go-spatial/proj.svg?branch=master)](https://travis-ci.org/go-spatial/proj)
[![Report Card](https://goreportcard.com/badge/github.com/go-spatial/proj)](https://goreportcard.com/report/github.com/go-spatial/proj)
[![Coverage Status](https://coveralls.io/repos/github/go-spatial/proj/badge.svg?branch=master)](https://coveralls.io/github/go-spatial/proj?branch=master)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/go-spatial/proj)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://github.com/go-spatial/proj/blob/master/LICENSE.md)


# proj: PROJ4, for Go!

Proj is a _selective_ and _on-going_ port of the venerable PROJ.4 project to
the Go language.

We do not intend to port all of PROJ.4: there is stuff in PROJ.4 that we'll probably never have a sufficient justification for bringing over. Likewise, we do not intend to do a verbatim port of the original code: naively translated C code doesn't make for maintainable (or idiomatic) Go code.


# Installation

To install the packages, preparatory to using them in your own code:

> go get -U github.com/go-spatial/proj

To copy the repo, preparatory to doing development:

> git clone https://github.com/go-spatial/proj
> go test ./...

See below for API usage instructions.


# Guiding Principles and Goals and Plans

In no particular order, these are the conditions we're imposing on ourselves:

* We are going to use the PROJ 5.0.1 release as our starting point.
* We will look to the `proj4js` project for suggestions as to what PROJ4 code does and does not need to be ported, and how.
* We will consider numerical results returned by PROJ.4 to be "truth" (within appropriate tolerances).
* We will try to port the "mathy" parts with close to a 1:1 correspondence, so as to avoid inadvertently damaging the algoirthms.
* The "infrastructure" parts, however, such as proj string parsing and the coordinate system classes -- _I'm looking at you, `PJ`_ -- will be generally rewritten in idiomatic Go.
* The `proj` command-line app will not be fully ported. Instead, we will provide a much simpler tool.
* All code will pass muster with the various Go linting and formatting tools.
* Unit tests will be implemented for pretty much everything, using the "side-by-side" `_test` package style. Even without testing all error return paths, but we expect to reach about 80% coverage.
* We will not port PROJ.4's new `gie` test harness directly; we will do a rewrite of a subset of it's features instead. The Go version fo `gie` should nonetheless be able to _parse_ all of PROJ.4's supplied `.gie` files.
* Go-style source code documentation will be provided.
* A set of small, clean usage examples will be provided.


# The APIs

There are two APIs at present, helpfully known as "the conversion API" and "the core API".

## The Conversion API

This API is intended to be a dead-simple way to do a 2D projection from 4326. That is:

**You Have:** a point which uses two `float64` numbers to represent lon/lat degrees in an `epsg:4326` coordinate reference system

**You Want:** a point which uses two `float64` numbers to represent meters in a projected coordinate system such as "web mercator" (`epsg:3857`).

If that's what you need to do, then just do this:

```
	var lonlat = []float64{77.625583, 38.833846}

	xy, err := proj.Convert(proj.EPSG3395, lonlat)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.2f, %.2f\n", xy[0], xy[1])
```

Note that the `lonlat` array can contain more than two elements, so that you can project a whole set of points at once.

This API is stable and unlikely to change much. If the projected EPSG code you need is not supported, just let us know.


## The Core API

Beneath the Conversion API, in the `core` package, lies the _real_ API. With this API, you can provide a proj string (`+proj=utm +zone=32...`) and get back in return a coordinate system object and access to functions that perform forward and inverse operations (transformations or conversions).

_The Core API is a work in progress._ Only a subset of the full PROJ.4 operations are currently supported, and the structs and interfaces can be expected to evolve as we climb the hill to support more proj string keys, more projections, grid shifts, `.def` files, and so on.

For examples of how to sue the Core API, see the implementation of `proj.Convert` (in `Convert.go`) or the sample app in `cmd/proj`.


# The Packages

The proj repo contains these packages (directories):

* `proj` (top-level): the Conversion API
* `proj/cmd/proj`: the simple `proj` command-line tool
* `proj/core`: the Core API, representing coordinate systems and conversion operations
* `proj/gie`: a naive implementation of the PROJ.4 `gie` tool, plus the full set of PROJ.4 test case files
* `proj/merror`: a little error package
* `proj/mlog`: a little logging package
* `proj/operations`: the actual coordinate operations; these routines tend to be closest to the original C code
* `proj/support`: misc structs and functions in support of the `core` package

Most of the packages have `_test.go` files that demonstrate how the various types and functions are (intended to be) used.


# Future Work

We need to support grid shifts, turn on more proj string keys, make the Ellipse and Datum types be more independent, port a zillion different projection formulae, the icky operation typing needs to be rethought, and on and on. Such future work on `proj` will likely be driven by what coordinate systems and operations people need to be supported: someone will provide a proj string that leads to successful numerical outputs in PROJ.4 but dies in proj.

We welcome your participation! See `CONTRIBUTING.md` and/or contact `mpg@flaxen.com` if you'd like to help out on the project.
