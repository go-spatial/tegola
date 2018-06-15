# geom
Geometry interfaces to help drive interoperability within the Go geospatial community. This package focuses on 2D geometries.

# Vendor

We should only vendor things needed for the CI system. As a library, we will not vendor our dependencies, but rather list them below:

* none.

The following code has been Vendored into the source tree:


* https://github.com/dhconnelly/rtreego [BSD 3 License](https://github.com/dhconnelly/rtreego/blob/master/LICENSE)
	We are keeping this internal, so that we can build an rtree implementation that uses geom types as it's base, but is build ontop of this.

