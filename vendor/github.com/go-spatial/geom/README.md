[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/go-spatial/geom)

# geom
Geometry interfaces to help drive interoperability within the Go geospatial community. This package focuses on 2D geometries.

# Vendor

We should only vendor things needed for the CI system. As a library, we will not vendor our dependencies, but rather list them below:

*  [go-sqlite3](https://godoc.org/github.com/mattn/go-sqlite3)
*  [uuid](https://godoc.org/github.com/pborman/uuid)
*  [errors](https://godoc.org/github.com/gdey/errors)
*  [proto](https://godoc.org/github.com/golang/protobuf/proto)
*  [p](https://godoc.org/github.com/arolek/p)

The following code has been Vendored into the source tree:


* https://github.com/dhconnelly/rtreego [BSD 3 License](https://github.com/dhconnelly/rtreego/blob/master/LICENSE)
	We are keeping this internal, so that we can build an rtree implementation that uses geom types as it's base, but is build ontop of this.

