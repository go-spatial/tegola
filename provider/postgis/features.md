# PostGIS feature queries

Raw feature queries use a separate, immutable source profile. Existing tile SQL
and tile decoding continue to use the tile configuration.

## Source admission

An ordinary `tablename` layer can publish features from one permanent physical
table. The ID must be an ordinary PostgreSQL smallint, integer, or bigint column
with a valid, immediate, single-column unique key. Stored identity and sequence
defaults are allowed; computed keys, views, inheritance, partitions, rewrite
rules, and row policies are outside this profile. Null IDs are excluded.

Unqualified table names are resolved once through the registration connection's
search path. Queries subsequently use the frozen, atomically quoted schema and
table names and verify the relation OID and full column catalog fingerprint.
Identifier folding requires a UTF8 server; identifiers exceeding the server's
`max_identifier_length` are rejected rather than truncated.

## Separate constrained SQL

A layer using custom tile SQL needs a separate `feature_sql` value. This accepts
one base-table SELECT with direct column projections, optional aliases, and a
restricted deterministic literal predicate. Joins, subqueries, functions,
expressions in projections, unions, and SQL paging are unsupported. All literals
are bound parameters. Geometry, identity, and temporal configuration names refer
to the selected output labels.

```toml
feature_sql = "SELECT f.id, f.geom, f.name AS title, f.observed FROM public.items AS f WHERE f.id >= 10"
id_fieldname = "id"
geometry_fieldname = "geom"
temporal_field = "observed"
temporal_storage = "unix_nanoseconds"
```

Ordinary layers use `fields` as their public property subset; absent or empty
means all eligible properties. A `feature_sql` projection defines the public
labels independently of tile `fields`. ID and geometry are not duplicated as
properties. Physical configured bounds columns and reserved `min_zoom`/`max_zoom`
fields are private even when aliased. Reserved names use PostgreSQL ASCII case
folding; configured bounds retain their exact physical catalog identity. Selected
temporal columns remain ordinary properties.
Unknown requested properties are errors.

Public scalar properties support PostgreSQL booleans, integer and finite floating
values, text, bytea, JSON, and JSONB. Other property types require an explicit
profile change; they are never silently omitted. Integer predicates retain exact
int64 values. Text predicates currently require catalog-proven C or POSIX
collation; floating comparisons, other collations, and decimal predicate profiles
are unsupported. Floating property output remains supported.

## Geometry and time

Native geometry requires an authoritative typmod with a supported linear geometry
family, source SRID, and XY or XYZ dimensions. Native output uses standard ISO WKB
and preserves Z. Geography, measured coordinates, curves, and surfaces are not
published. Explicit raw WKB, WKT, and MOS profiles are also supported; MOS is XY.
XYZ sources require an explicit height reference and a supported canonical height
projection. Corrupt source geometry or inconsistent source metadata is a source
data error.

After complete dimensional and geometry validation, wholly empty raw or native
geometry is delivered as absence (`null`). An empty child within a nonempty
collection does not change its spatial meaning or hide an invalid child.

A valid XYZ source surface that becomes nonplanar in the requested bbox CRS
returns an unsupported spatial profile. A malformed source surface remains a
source data error.

Temporal mappings use `temporal_field` or `temporal_start_field` and
`temporal_end_field`, with `temporal_storage` set to `unix_seconds`,
`unix_milliseconds`, `unix_microseconds`, or `unix_nanoseconds`. The mapped source
columns must be ordinary integers. Query bounds retain their exact fractional
precision and leap-second semantics when compared with the source tick scale.

## Snapshot and paging

Each query runs in a read-only Repeatable Read transaction, holds an Access Share
lock on the physical table, and verifies catalog admission before callbacks.
Rows are read in bounded ID keyset chunks. Exact bbox and temporal filtering occurs
before logical offset and limit. Same-SRID native geometry can use an indexed bbox
candidate; cross-CRS and raw profiles use an ordered scan and exact query-frame
geometry filtering. Large scans may therefore be expensive. Total match counts
are not reported; an additional exact match establishes `hasMore`.

## Live validation

The opt-in tests require `RUN_POSTGIS_TESTS=yes` and an explicit writable `PGURI`.
They create and remove isolated schemas and cover the shared query contract,
native XYZ export, concurrent mutation, and DDL protection. Without a live database
these tests skip and provide no native or live parity evidence.

See also [provider configuration](README.md) and the
[feature query contract](../../docs/provider-contract.md).
