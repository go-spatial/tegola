[← API reference](api.md) · [Back to README](../README.md) · [Provider evidence →](provider-matrix.md)

# WFS/WFS-T scope and operating limits

Tegola provides WFS read/transaction adapters and an experimental OGC API
Features CRUD implementation. Implementation, unit tests, native integration
runs, and standards conformance are different levels of evidence. Neither this
file nor the presence of a handler establishes full OGC conformance or production
acceptance. See [provider validation](provider-matrix.md) for coverage and provider versions.

## Publication and concurrency

Writes require explicit per-collection configuration. Use production authentication
and require strong If-Match preconditions on existing features. A representation
hash alone is not a substitute for an atomic revision check. External database
writers must participate in revision management; otherwise concurrent-edit
protection cannot be promised.

The stock CLI rejects production-mode writes because it does not install an
authenticator. Custom hosts must supply authentication to both protocol adapters;
the explicit anonymous `dev` profile is for disposable trusted environments.
See [configuration](configuration.md#explicit-wfs-and-write-publication).

LockFeature is unavailable until a physical-feature guard can be enforced by
all mutation paths and processes. Do not interpret an in-memory token store as a
distributed database lock.

A write-enabled server uses the configured server-side tile cache for ordinary
viewing and emits HTTP `no-store` so browsers do not hide committed changes.
Send `X-Tegola-Editor-Active: true` (or `1`) for an ordinary tile request to bypass
server-side caching while editing. The header grants no maintenance or write rights.
Authenticated explicit tile-cache maintenance remains available independently.

Successful and uncertain commits invalidate all maps in that router by advancing
its cache generation. Concurrent old renders can only populate the old generation.
Writable caches start cold after each restart; old generation files require backend
expiry or operational cleanup. This is process-local invalidation: external SQL
changes and other reader processes are not coordinated. Do not use this mechanism
as distributed invalidation for multiple replicas serving editable data.
## Embedded attribute editor

The viewer includes a limited source-feature attribute editor. Its default API
path is `features`; change the same-origin path when the deployment configures a
different Feature API base path. It loads the mutation schema, then loads a feature
by decimal ID together with its ETag. Saving existing features sends JSON Patch
operations only for changed attributes: geometry is retained. NULL, omitted
properties and empty strings are distinct. Undo, redo and cancel act on the local
attribute draft. Create accepts a manually entered GeoJSON geometry; it is not a
map drawing tool.

A 412 keeps the draft and disables saving until the source is explicitly loaded
again. Copy the draft before reloading; there is no automatic conflict merge.
Network/server failures preserve the draft and stop further writes because the
commit outcome can be unknown. Verify the source before reconnecting. There is
no automatic retry of a potentially committed write.

A confirmed commit with failed source readback returns a minimal successful
response and `Tegola-Commit-Status: committed`. The editor distinguishes this
case from an unknown outcome and never retries the write. Successful saves
refresh the viewer's loaded vector sources.

The editor rejects unsafe JavaScript integers and decimal tokens exceeding its
conservative precision limit. Use an external lossless client for large IDs or
exact decimal values; the editor does not claim a complete uint64 editing profile.
Geometry drawing, geometry undo/redo, and collaborative conflict resolution remain
outside the embedded profile. Existing authentication must be provided by the
same-origin deployment; this UI is not an authentication management console.

## Provider and protocol limits

PostGIS, MySQL/MariaDB and GeoPackage have mutation implementations. Their native
test status must be recorded separately per database version, geometry encoding,
CRS and operation; unit tests do not establish this matrix. HANA writes are not
implemented and must not be admitted. Do not infer MOS, custom CRS or XYZ write
support from the availability of those read formats.

MySQL and the GeoPackage provider maintain configured/discovered four-column
bounds mappings for admitted raw MOS table writes. See the
[MOS write contract](geometry-formats.md#mos-writes-with-separate-bounds-columns)
for mapping, quantization and nullability requirements. Derived bounds for other
formats and PostGIS remain unsupported. Native GeoPackage RTree indexing is
handled separately; derived bbox columns and a native spatial index are not
interchangeable. MySQL 5.5 additionally needs the explicit
[legacy identity opt-in](configuration.md#mysql-55-table-identity).

Both Merge Patch and the implemented JSON Patch subset are accepted by the REST
adapter. Unsupported JSON Patch operations, filter expressions, spatial profiles
and lock operations must fail explicitly. The bounded WFS filter, sort and hits
implementation is not a declaration of full FES or WFS conformance.

Mutation schemas preserve storage nullability, including the geometry column.
An explicit null is rejected for a non-nullable column even when it has a server
default; omission and null are different inputs. Date-time properties must use
the accepted RFC3339 timestamp syntax, including announced positive leap seconds
validated consistently with query literals. DescribeFeatureType reflects geometry
nullability through both `minOccurs` and `nillable`. Invalid values fail validation before
the provider transaction. Create requires a generated primary key; no client ID
is implicitly substituted for a missing database default. For PostGIS, only an
identity or validated sequence-generated key establishes create capability;
a constant or arbitrary default does not. Update/delete-only admission remains
separate from this create restriction.

WFS scalar properties and nullable geometry accept explicit XML NULL in Insert
and Replace feature
fields (`<app:note xsi:nil="true"/>`) and Update values
(`<wfs:Value xsi:nil="true"/>`), with `xsi` bound to
`http://www.w3.org/2001/XMLSchema-instance`. The boolean form `1` is also
accepted. The property must be nullable; a nil element cannot contain text or
child elements. Geometry NULL is mapped to a native SQL NULL only when the
selected storage geometry is nullable. Update with an omitted `Value` remains
unsupported. An empty `Value` continues to mean an empty string for string properties, rather than NULL;
`xsi:nil="false"` and `"0"` preserve ordinary value parsing. These are explicit
implementation limits, not a claim of support for every WFS NULL encoding.

Create and replacement require a geometry value when the storage geometry is
non-nullable, including WFS input that omits the geometry property. A partial
update with no geometry change preserves the existing geometry.

Part 4 follows a draft-derived implementation contract; no final Part 4 conformance
class is declared merely because writes are enabled. The OpenAPI document describes
installed methods and media types; conformance needs independent protocol evidence.

## Operational acceptance

Before enabling production writes, verify migration privileges and upgrade behavior,
backup and actual restore, revision state after restore, write-disable procedures,
connection pool limits and timeouts, audit retention, external-writer policy and
reconciliation of unknown commit outcomes. A successful build and a local fixture
run do not close these operational requirements.

## See Also

- [Provider evidence](provider-matrix.md) — native versions, formats and verification boundaries.
- [Write operations](operational.md) — schema preparation, recovery and commit correlation.
- [API reference](api.md) — routes, validation and protocol behavior.
