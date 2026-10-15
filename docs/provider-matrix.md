[← Write scope](wfs-scope-limitations.md) · [Back to README](../README.md) · [Write operations →](operational.md)

# Provider write evidence

## Native validation coverage (2026-10-04)

These rows describe specific native validation runs. They are not a
blanket certification of all versions, CRS, encodings or operations. Preserve the
exact commit/build identity and raw output with each release gate.

| Provider/profile | Native environment | Verified coverage | Remaining boundary |
|---|---|---|---|
| MySQL native geometry | MySQL 8.4.11; strict SQL mode; REPEATABLE-READ | Insert, longitude/latitude axis check at 120/35, revision read, identical replace, hidden property/schema admission, physical alias CAS, delete/tombstone, future-schema refusal | Other versions, all geometry encodings and real commit-fault recovery not established |
| PostGIS native geometry | PostgreSQL 16.15; PostGIS 3.5.7; GEOS 3.14.1; PROJ 9.8.1 | Insert, guarded replace/update, revision read, hidden properties, alias CAS, delete, future-schema refusal | Other server/profile combinations and full production acceptance not established |
| GeoPackage | Native SQLite writer with an independently created GDAL fixture copy | Real HTTP PATCH/PUT/CAS, geometry bytes, exact decimal text, child relations, RTree/extents, trigger rollback, delete cascade, WFS 2 update, untouched polygon/multipart/Z, source immutability and integrity | This does not establish XYZ writing, every custom CRS or restore/fault acceptance |
| MySQL raw MOS | MySQL 8.4.11; LONGBLOB; EPSG:3857; explicit precision 2, units m | Native six-family mutation matrix, quantization, annotation-byte preservation, attribute filter/bbox, polygon-hole exclusion, replacement/delete and 4326→3857 geometry write | That run had no derived bbox columns; no MariaDB or default-precision inference |
| GeoPackage provider / raw SQLite MOS | SQLite BLOB; EPSG:3857; explicit precision 2, units m | Same native mutation matrix, plus real HTTP GET/PATCH/stale 412/geometry PATCH/direct SQL readback/DELETE→404 | This raw MOS storage is not GeoPackage binary geometry; that run did not test bounds writes |
| Bounds-backed MOS | MySQL 5.5.29/InnoDB and GeoPackage provider/raw SQLite; custom `etmerc` CRS | Point, line and polygon HTTP/browser CRUD, separate integer bounds readback, conditional conflicts; native six-family tests | Canonical MapplGIS tables have narrower update/delete admission; see the combined-profile coverage below |
| MariaDB | Not established by these mutation runs | Shared implementation and unit coverage only | Native MariaDB acceptance remains NOT_RUN |
| HANA | No write implementation | Read-only provider | Writes unsupported; no writer acceptance claimed |

The embedded browser exercised source-feature attribute load/save against the
GeoPackage server, undo/redo, empty string vs NULL, preservation of existing
geometry, and two-tab stale-validator conflict with the local draft retained.
The final built viewer also saved successfully after map-source refresh support
was added. Geometry drawing and large-number editing are not part of that result.

Additional native tests passed on the listed MySQL/PostGIS versions:
generated-key create admission while manual-key update remains available,
transaction ID matching the committed audit row, and deterministic revision-first
lock ordering for update/replace/delete, first revision creation and unguarded
writes. MySQL additionally verifies startup migration and admission with a
restricted DML-only runtime role after administrator preparation. The error-path
receipt test commits an already closed native transaction; it verifies retained
correlation, not a real connection loss during COMMIT or a lost acknowledgement.

## Transaction and validation guarantees

The coordinator distinguishes committed, known-not-committed and unknown outcomes.
An unknown response retains available transaction correlation and requires source
reconciliation before retry. An injected PostGIS acknowledgement failure after a
real commit verifies durable data with an unknown receipt; this is not a wire-level
connection cut. GeoPackage failure-path regressions verify that failed Apply or
metadata work cannot later commit a partial transaction.

WFS tests cover QName resolution, typed filter literals, nullable geometry and
public operation URLs for all supported versions. Conditional-write tests cover
revision-first lock ordering, physical aliases and representation-specific ETags.
These checks describe application contracts, not official standards certification.
Native PostGIS DATE, TIMESTAMP, TIMESTAMPTZ and NUMERIC source properties remain
unsupported; parser-level date/time/decimal binding does not admit those columns.

## Reproduce the provider gates

Use disposable databases: these tests create/modify fixture and service tables.
Supply credentials through the environment and keep them out of logs:

- `TEGOLA_REVIEW_MYSQL_DSN`: used by `TestReviewMySQLNativeMutation` in
  `provider/mysql/mutation_review_test.go`.
- `TEGOLA_REVIEW_MYSQL_ADMIN_DSN`: privileged disposable-server connection for
  `TestFollowupMySQL*` in `provider/mysql/mutation_followup_test.go`. These tests
  create isolated databases/users and inspect `performance_schema`; an ordinary
  database-scoped test user is insufficient. If omitted, the helper falls back
  to `TEGOLA_REVIEW_MYSQL_DSN`, which must then provide those privileges. CI uses
  its disposable MySQL administrator for this fixture, not a production account.
- `TEGOLA_REVIEW_POSTGIS_DSN`: used by `TestReviewPostGISNativeMutation` in
  `provider/postgis/mutation_review_test.go` and `TestFollowupPostGIS*` in
  `provider/postgis/mutation_followup_test.go`; the follow-up creates isolated
  schemas, so its role needs schema-creation privileges.
- `TEGOLA_REVIEW_GPKG`: optional independent source fixture override used by
  `TestNativeGeoPackageHTTPPreservation` in `server/native_gpkg_review_test.go`.
  With CGO enabled the test defaults to bundled `testdata/wfs/editing-fixture.gpkg`
  and works on a copy; it does not require this environment variable.

Run `scripts/native-test.sh` with the intended environment, or invoke the named Go
tests directly. Missing required MySQL/PostGIS environments produce SKIP, never native PASS.
The GeoPackage tests require CGO and a working C compiler. Build the locked UI
assets before compiling a server for browser acceptance.

### MOS-specific gates

The MOS matrix uses explicit `geometry_format = "mos"`, EPSG:3857,
`mos_precision = 2`, and `mos_units = "m"`. MySQL stores raw MOS in a LONGBLOB;
the GeoPackage provider test uses a raw SQLite BLOB table, not GeoPackage binary
geometry. Neither fixture has derived MINX/MAXX/MINY/MAXY columns. These tests
must not be relabeled as a bounds-backed MapplGIS write profile.

The passing mutation cases cover Point, LineString, Polygon with a hole,
MultiPoint, MultiLineString and MultiPolygon with a hole. An attribute-only
update preserves native 10-byte-header MOS and appended opaque annotations.
Geometry replacement is checked after decode and quantization; the explicit
4326 point `(0.0001, 0.0002)` is stored as 3857 `(11.13, 22.26)` under the tested
precision. The matrix exercises insert/read/filter/bbox/replace/delete and does
not claim every CRS or automatically selected precision.

Enable CGO (`CGO_ENABLED=1`) with a working C compiler, then run the
self-contained raw SQLite MOS matrix:

```shell
go test ./provider/gpkg -run '^TestNativeMOS(MutationMatrix|BoundsProfile)$' -count=1 -v
```

For MySQL, set `TEGOLA_MOS_MYSQL_DSN` to a disposable database using the Go MySQL
driver DSN syntax, then run:

```shell
go test ./provider/mysql -run '^TestNativeMOS(MutationMatrix|BoundsProfile)$' -count=1 -v
```

`TEGOLA_MOS_MYSQL_DSN` overrides the MOS test database. When unset, both direct
tests and `scripts/native-test.sh` reuse `TEGOLA_REVIEW_MYSQL_DSN`. If neither
variable is set, the MySQL MOS tests report SKIP, not PASS. The test role needs
fixture/service-table creation privileges; these commands are not read-only
probes against a production dataset. Bounds-backed reads and mutation admission
are covered by separate cases; run the full MOS suite for the intended profile.

The self-contained HTTP regression uses the GeoPackage provider's raw MOS table:

```shell
go test ./server -run '^TestNativeMOSHTTPRoundtrip$' -count=1 -v
```

It verifies source HTTP reads, attribute PATCH with byte preservation, stale
If-Match rejection, CRS-aware geometry PATCH checked through direct SQL MOS
readback, and deletion followed by 404. This fixture is distinct from the combined MySQL/SQLite browser run described below.

### MOS bounds and custom CRS validation

The implementation supports transactional four-column MOS bounds maintenance
for direct MySQL/InnoDB and GeoPackage-provider raw SQLite tables. It uses
immutable custom `etmerc` horizontal transforms with explicit datum semantics.
These source capabilities do not retroactively extend the native evidence above.
The custom projection unit/race suite compares synthetic Bessel three/seven-parameter
transforms, prime-meridian offsets and units with PROJ 9.5.1.

The local combined-profile run exercised six published collections across
MySQL 5.5.29/InnoDB and the GeoPackage provider's raw SQLite MOS storage, with
custom `etmerc` coordinates and separate integer bounds. HTTP and browser checks
covered points, lines and polygons on both providers, CRUD, bounds readback and
conditional conflicts. Native provider tests separately cover all six ordinary
geometry families and attribute-only byte preservation. This run used a local test client;
neither its deployment files nor private source data are repository fixtures.
The final exact-binary replay repeated browser CRUD for all three geometry
families across the six collections, verified actual WFS geometry XML requests,
read all three supported WFS versions, and reported zero JavaScript errors.
MySQL DOUBLE query predicates remained unsupported in that profile; property output does not establish numeric filtering. Browser-created test records were removed. Final direct-SQL checks confirmed unchanged baseline geometry/bounds, no temporary rows and valid SQLite integrity.

Canonical MapplGIS tables have a narrower existing-row profile: protected
SystemInfo/MUID/ObjectType/style metadata, preserved geometry family, and no
geometry rewrite when opaque/header payload cannot be retained. Create remains
unsupported there. Ordinary raw MOS tables support the separately admitted full
CRUD profile. See [geometry write limits](geometry-formats.md#mos-writes-with-separate-bounds-columns).

For current native provider regressions use the disposable DSN described above:

```shell
go test ./provider/mysql -run '^TestNativeMOS' -count=1 -v
go test ./provider/gpkg -run '^(TestMOSNative|TestMOSAuxiliary|TestMOSSystemInfo|TestCanonicalMapplGIS)' -count=1 -v
```

The MySQL 5.5 identity-specific gate is `TestNativeMySQL55LegacySnapshot`. The
self-contained SQLite gates include `TestNativeMOSBoundsProfile` and
`TestCanonicalMapplGISMutationProfile`. Missing database credentials are SKIP.

MySQL 5.5 uses the default-disabled `allow_legacy_table_identity` compatibility
option. Its metadata identity cannot detect an identical-shape drop/recreate in
the same creation-time second; this is not the modern physical-table-ID contract.
See [configuration](configuration.md#mysql-55-table-identity).

## Explicit exclusions

Derived bbox writes remain rejected outside the admitted MySQL/SQLite MOS profile. A native GeoPackage RTree has separate supported
maintenance and is not equivalent to a derived bbox-column profile. External
writers that bypass revision management are outside conditional-write guarantees.

These fixtures do not establish wire-level connection-loss-during-COMMIT recovery, backup/restore
acceptance, an official WFS/Part 4 ETS, or distributed/CDN invalidation. Keep these
as NOT_RUN requirements for any deployment that depends on them. See
[write limits](wfs-scope-limitations.md) and [operational procedures](operational.md).

## See Also

- [Write scope](wfs-scope-limitations.md) — admitted operations and exclusions.
- [Geometry formats](geometry-formats.md) — MOS quantization and read-side bounds.
- [Write operations](operational.md) — recovery requirements beyond fixture tests.
