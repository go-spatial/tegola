# HANA MVT provider

Use provider type `mvt_hana` to encode vector tiles inside the database.
The implementation and configuration reference live in
[`provider/hana`](../../provider/hana/README.md); this directory retains
the package documentation entry point rather than a second copy of that contract.

```toml
[[providers]]
name = "tiles"
type = "mvt_hana"
uri = "hdb://user:password@host:443"
```

Add provider layers and maps according to the linked configuration reference.
Use database-native geometry and database-side SRS definitions. Raw
`geometry_format`/MOS decoding and synthetic `crs_defn` are contracts of the
standard providers, not the database-side MVT path.

Pixel-width/height tokens use source-CRS units and actual tile pixel dimensions;
scale denominators follow the [shared CRS contract](../../docs/crs.md#scale-tokens).
They do not universally assume meters or 256-pixel custom tiles.

For live database test setup see the [provider testing instructions](../../provider/hana/README.md#testing)
and the [workflow](../../.github/workflows/on_pr_push.yml).
