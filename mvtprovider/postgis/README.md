# PostGIS MVT provider

Use provider type `mvt_postgis` to encode vector tiles inside the database.
The implementation and configuration reference live in
[`provider/postgis`](../../provider/postgis/README.md); this directory retains
the package documentation entry point rather than a second copy of that contract.

```toml
[[providers]]
name = "tiles"
type = "mvt_postgis"
uri = "postgres://user:password@localhost:5432/database"
srid = 3857

[[providers.layers]]
name = "landuse"
geometry_fieldname = "geom"
id_fieldname = "gid"
sql = "SELECT ST_AsMVTGeom(geom, !BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"
```

This example assumes source geometry in EPSG:3857. Custom MVT SQL must return
`ST_AsMVTGeom` geometry, rather than the `ST_AsBinary` output used by the
standard provider. For EPSG:4326 sources, configure `srid = 4326`, transform
both the geometry and `!BBOX!` to EPSG:3857 inside `ST_AsMVTGeom`, and keep
the WHERE comparison in the source CRS. Add maps according to the linked
configuration reference.
Use database-native geometry and database-side SRS definitions. Raw
`geometry_format`/MOS decoding and synthetic `crs_defn` are contracts of the
standard providers, not the database-side MVT path.

Pixel-width/height tokens use source-CRS units and actual tile pixel dimensions;
scale denominators follow the [shared CRS contract](../../docs/crs.md#scale-tokens).
They do not universally assume meters or 256-pixel custom tiles.

For live database test setup see the [provider testing instructions](../../provider/postgis/README.md#testing)
and the [workflow](../../.github/workflows/on_pr_push.yml).
