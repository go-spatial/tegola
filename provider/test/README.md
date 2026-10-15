# Provider test harness

Provider tests share setup through `provider/test/fixture`. This package does
not import concrete providers or register mock providers, so it can be used by
both internal and external provider tests without import cycles or registry
side effects. Production provider interfaces and behavior are unchanged.

## Setup helpers

| Helper | Contract |
| --- | --- |
| `fixture.Config(base, overrides, layers)` | Shallow-copy the base config, apply overrides, then append layers to the resulting `"layers"` entry. Nested maps remain shared; appending does not mutate the base layer slice's backing array. Provider-specific defaults and base selection stay with the caller. |
| `fixture.Tile` | Explicit Z/X/Y, SRID, unbuffered `Bounds`, and `BufferedBounds`. Neither extent falls back to the other. Nil and non-finite bounds are preserved for error-path tests. Use the existing `provider.NewTile` when ordinary slippy-tile bounds are wanted. |
| `fixture.OpenDB(t, driver, dsn, statements...)` | Open an already-registered SQL driver, execute setup statements in order, and close the database with `t.Cleanup`. Open, setup, and close errors fail the test. The caller owns schema SQL, temporary paths, and driver imports/build tags. |
| `fixture.OpenSQLRows(t, fixture.SQLRows{...})` | Return a real `*sql.DB` backed by fixed in-memory rows, with a fresh cursor per query and automatic cleanup. No global driver registration or live server is needed. |
| `fixture.DriverRows(rows)` | Convert shared fixture `int` cells to driver `int64` cells; preserve other values, including NULL and malformed blobs. |

`SQLRows` accepts `Columns`, `Rows`, and optional `TypeNames`. Unspecified type
names are empty. Optional `QueryLog`, `ContextLog`, and `Closes` retain query
text, query contexts, and driver-row close counts. Logs are for sequential
tests; do not inspect them concurrently with queries. Keep the result
configuration immutable while the database is in use.

The SQL fixture deliberately does **not** evaluate predicates, filter rows,
validate SQL syntax, or interpret arguments. This matters for tests proving
that a provider's in-memory geometry filter is necessary. Prepared statements
and transactions return errors rather than pretending to succeed. Use a
backend-specific stub for retry sequences or driver-specific protocols.

For file-backed SQLite fixtures, construct the path with `t.TempDir()` before
calling `OpenDB`, so cleanup closes the database before removing the directory.
Register provider cleanup after fixture setup to release provider-owned handles
first. A caller may explicitly close the setup database before opening the
provider; `database/sql` permits the later cleanup close.

## Mock tile provider

Importing `provider/test` registers the existing `test` and `mvt_test` mock
providers. `TileProvider.MVTForLayers` has two explicit modes:

- By default, return `MVTTile` verbatim with no error, including for canceled
  contexts, unknown/requested layer names, and a nil receiver (nil bytes).
  The bytes are opaque: this mode neither filters nor renames encoded layers.
- When `MVTForLayersFunc` is set, pass the original context, tile, parameters,
  and ordered layer requests to it and return its exact bytes/error. The
  callback takes precedence over `MVTTile` and owns cancellation, input
  assertions, layer selection, and failure injection.

`NewMVTTileProvider` loads the optional configured `test_file` once and releases
the file handle. A nil config constructs an empty canned provider; a non-nil
config must supply `test_file`, and configuration/read errors are returned.

## Migration boundaries

HANA and MySQL use the shared SQL-row fixture for inspection, sampling, and
fixed-result tests; their fixture-row conversion implementations were removed.
HANA and PostGIS share config assembly. GPKG and HANA use the explicit tile
fixture instead of three separate tile implementations. GPKG's raw and
metadata database builders share SQL open/schema/cleanup setup.

Provider-specific layer construction, schema SQL, fixture files, binary
format builders, decoding assertions, and live connection setup stay local.
PostGIS's `pgx.Rows` stub and MySQL's stateful retry/no-query drivers implement
different protocols and are not replaced by a generic SQL fake. The existing
`mosfixture` package still supplies common geometry-contract data. The
`crsconfig`, `geometrycodec`, and `mapplgis` suites retain their pure contract
fixtures; the collection/empty-collection mocks retain their distinct geometry
behavior. `mvtprovider` contains documentation, not another test setup.

All existing assertions, fixture payloads, CGO build tags, and live-database
gates remain in place. Live tests still require `RUN_POSTGIS_TESTS=yes`,
`RUN_HANA_TESTS=yes`, or `RUN_MYSQL_TESTS=yes` and their existing server setup.
The shared fixture never enables or skips those tests.
