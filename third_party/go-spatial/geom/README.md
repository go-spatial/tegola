[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://pkg.go.dev/github.com/go-spatial/geom)

# Packages

## package geom
Geometry interfaces to help drive interoperability within the Go geospatial community. This package focuses on 2D geometries.


## package encoding

Encoding package describes a few interfaces and common errors for various sub packages implementing
different encoders.

# Dependencies

Dependencies are managed through `go mod` with the execption of one package:

* https://github.com/dhconnelly/rtreego [BSD 3 License](https://github.com/dhconnelly/rtreego/blob/master/LICENSE)
	We are keeping this internal, so that we can build an rtree implementation that uses geom types as it's base, but is build ontop of this.

