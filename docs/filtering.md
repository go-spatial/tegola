[Documentation index](README.md)

# Queryables and CQL2 filtering

The opt-in feature API publishes a collection-specific Queryables schema at
`/features/collections/{collectionId}/queryables`, using the configured feature
base path. The schema uses Draft 2020-12 and `application/schema+json`. Its `$id`
is the canonical resource URL without a query string; `additionalProperties`
is false. Collections with an available catalog link to this resource with relation
`http://www.opengis.net/def/rel/ogc/1.0/queryables`.

Queryables describes eligible public scalar properties. Geometry storage,
bounds, operational fields and feature identity are not automatically exposed.
The catalog derives from proven physical types and published projection aliases,
and can be empty. Check each collection's schema before building a filter.
Missing optional Queryables capability leaves ordinary Core requests available
and returns 501 for Queryables or filtering operations.

## Request syntax

Send a CQL2 text expression in the items resource's `filter` parameter. Encode
the expression as a URL query value. `filter-lang=cql2-text` explicitly selects
the supported text language; omitting it with a filter selects the same language.

```text
/features/collections/roads/items?filter=population%20%3E%3D%201000&filter-lang=cql2-text
/features/collections/roads/items?filter=name%20%3D%20%27Central%27
```

These examples require the named properties to appear in that collection's
schema. Filters combine with the configured source selection, bbox and Core
datetime constraints by AND, before paging and exact counts. Paging links retain
the validated filter and other request constraints.

The text transport catalog contains only aliases expressible as quoted CQL2
identifiers. Quoting permits identifier keywords; it does not turn arbitrary
names into identifiers. Space-containing, digit-first or hyphenated aliases are
omitted from this catalog while remaining ordinary public feature properties.
Eligible Unicode identifiers, including CJK names, retain their exact spelling.

The implemented expression profile supports:

- Standalone `TRUE` and `FALSE`, and `AND`, `OR`, `NOT` with parentheses.
- Property-left, literal-right comparisons: `=`, `<>`, `<`, `<=`, `>`, `>=`.
- Property `IS NULL` and `IS NOT NULL`.
- String, boolean, exact decimal/exponent numeric, `DATE('2020-01-01')` and
  `TIMESTAMP('2020-01-01T00:00:00Z')` scalar literals, subject to catalog types.

Keywords are ASCII case-insensitive. String literals use single quotes; double
a single quote inside the literal. Quoted property identifiers use double quotes.
Literal `NULL`, property-to-property comparisons, literal-left comparisons,
LIKE, IN, BETWEEN, arithmetic, functions, spatial predicates and advanced
temporal predicates are outside this profile. CQL2 JSON and `filter-crs` are
also unsupported. DATE/TIMESTAMP literal parsing does not establish native
database date/time queryables.

Unknown or private properties, incompatible literal types, invalid syntax,
repeated or blank parameters, and an orphan `filter-lang` return 400. Error
responses omit filter values and database details.

## Comparison semantics

Numeric comparison preserves mathematical decimal precision; it does not round
request literals to binary floating point or a column's scale. Integer and
number properties share this comparison family. Fractional and out-of-range
bounds remain valid comparisons against integers.

Strings compare in Unicode scalar ordinal order, including case, accents,
combining characters and trailing spaces. No trimming or Unicode normalization
is applied. Booleans order false before true. Comparisons with a source NULL
produce UNKNOWN; AND, OR and NOT preserve three-valued logic. Only TRUE selects
a feature. Use `IS NULL` to select absent values.

Eligible, explicitly published POSIX temporal aliases remain integer queryables in their configured
storage units. Core `datetime` retains its own exact instant/interval semantics;
an integer property comparison does not change those units. CQL2 timestamps use
UTC `T`/`Z` syntax, preserve fractional tails and validate known leap-second
positions; Core RFC3339 offset syntax is a separate request grammar.

## Provider eligibility and limits

Physical types require separate admission. Unproved floating-point, padded text
and native date/time profiles are omitted from catalogs, rather than inferred
from sampled values. See the backend guides for admitted types and source
integrity checks. GPKG custom feature SQL remains unsupported.

| Backend | Eligible scalar profiles |
| --- | --- |
| GeoPackage | Proven integer declarations with signed SQLite integer storage; BOOL/BOOLEAN with checked 0/1 storage; finite INTEGER/REAL numbers under numeric declarations; UTF-8 TEXT with filtered source integrity checks. |
| MySQL/MariaDB | Signed/unsigned integer widths, including UINT64; DECIMAL precision 1–65 and scale 0–30 within precision; BIT(1); utf8mb4 VARCHAR/TEXT families. |
| PostGIS | int2/int4/int8, native boolean, text/varchar. |
| HANA | Proven integer widths, DECIMAL precision through 38 with exact result metadata, native BOOLEAN, NVARCHAR on the admitted server profile. |

These entries apply to published direct properties in otherwise eligible source
profiles. A supported physical declaration alone does not publish a private
field or make feature identity a queryable.

Providers compile the neutral expression using private field mappings, enum
operators and bound values. They revalidate their own catalog; a caller-supplied
catalog cannot authorize physical identifiers. There is no ordinary filter
fallback that loads the entire collection into Go. SQL source integrity probes
or casts can still scan data; filtering does not promise use of a spatial or
scalar index.

Requests are bounded to 64 KiB expression text, 8192 tokens, 4096 AST nodes and
depth 32. Property names allow 1024 bytes, literal text 16 KiB, numeric literals
1024 digits and absolute exponent 4096, and timestamp fractions 1024 digits.
Aggregate AST property/literal text is bounded to 64 KiB. Catalogs allow 4096
entries and 64 KiB aggregate property names. Limits reject input without
truncation.

Filtering/CQL2 conformance classes are not advertised. The Feature API separately
advertises its admitted Core, GeoJSON, HTML and OpenAPI implementation classes,
and Part 2 CRS when every collection qualifies. These declarations are not an
OGC certification; see [conformance status](api.md#errors-and-conformance-status).

## See Also

- [API reference](api.md) — endpoints, paging, errors and Core datetime
- [Provider contract](provider-contract.md) — source publication boundaries
- [GeoPackage](../provider/gpkg/README.md), [MySQL/MariaDB](../provider/mysql/README.md),
  [PostGIS](../provider/postgis/README.md), [HANA](../provider/hana/README.md) — physical eligibility
- [Typed filtering decision](architecture/decisions/ADR-0009-typed-feature-filtering.md)
