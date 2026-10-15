# Native editing fixture

`editing-fixture.gpkg` is an independently generated synthetic GDAL/Fiona GeoPackage. It contains
three points, a polygon with a hole, a multipart line, an XYZ control point,
native RTree triggers, child rows and update/delete audit triggers. All names,
tenant IDs and exact-number strings are deliberate test values.

`fixture-manifest.json` records the expected original features, generating
library versions and SHA-256. Keep the fixture immutable. The HTTP regression
test copies it to `t.TempDir()` before opening a writer and checks foreign keys,
RTree, metadata, rollback and untouched control geometries. No running data
source is modified. `TEGOLA_REVIEW_GPKG` can select another compatible fixture.

The corpus is independently generated test input, not evidence that Tegola has
passed an OGC conformance suite.
