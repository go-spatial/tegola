# HANA raw feature query implementation

The feature executor reserves one connection and starts one read/write,
repeatable-read transaction. It obtains an EXCLUSIVE lock on the qualified source
before catalog revalidation and holds that lock through all reads and callbacks.
It issues no DML. Cleanup rolls back and unconditionally discards the physical
session, including setup failures; go-hdb does not restore access mode or isolation
on rollback. The earliest caller deadline or a 30-second deadline bounds database
work. A synchronous callback must cooperate with cancellation; its execution cannot
be forcibly stopped. Cancellation releases the server transaction and prevents
subsequent reads and callbacks.

A query-scoped cancellation watcher closes only the reserved connection's captured
physical socket, interrupting blocked row and LOB reads without waiting for a
driver mutex. Socket association must be unique and proven; an unavailable or
ambiguous association fails the feature query. The watcher is stopped and joined
before normal physical discard. Tile queries do not arm this watcher.

## Source profiles

The admitted profile uses a persistent row or column table with a
catalog-proven ordinary single-column unique integer identity. Nullable identity
rows are excluded; negative identities are source errors. Stored identity columns
are allowed. Computed identities, virtual/external tables, masked columns,
structured privileges and temporal tables are unavailable.

WKB and WKT support the declared XY, XYZ and mixed profiles. MOS supports its six
XY spatial families and SQL NULL; malformed or wholly empty MOS bodies are errors.
Native `COORD_DIMENSION=2` supports XY. Permissive native dimension 4 requires an
explicit dimensional declaration, and every encountered ISO body is checked
strictly before filtering or pagination. M/ZM and dimensional mismatch are source
errors. Topological `ST_Dimension` and samples are not dimensional proof.

Native source CRS admission uses complete exact catalog tuples for canonical 4326
and its proven planar-coordinate equivalent 1000004326. Other native CRS definitions,
including 3857, are currently unsupported. The frozen source interpretation and
canonical adapter do not use mutable database projection or bounding-box functions.
Native queries only export stored ordinates with `ST_AsBinary`; complete CRS
fingerprints are revalidated with the catalog. The source table lock does not claim
to lock the global SRS registry.

XYZ and mixed XY/XYZ sources require an explicit supported height reference and
canonical horizontal projection. The physical source SRID is retained separately
from HANA's planar tile label. Exact query-frame geometry predicates run before
logical offset, page length and lookahead. Integer POSIX temporal fields retain
exact query fractions and leap-second semantics for seconds, milliseconds,
microseconds and nanoseconds.

For ordinary tables, configured `fields` selects published properties. With no
configured subset, supported public columns are published. `feature_sql` uses the
shared constrained parser; its direct projections define public output labels.
Identity, geometry and configured private bounds/meta columns are suppressed by
physical lineage even when projected under another name.

Selected public SQL NULL values remain present properties with Go `nil` and JSON
`null`. Private and unselected fields remain absent. Mapped temporal fields remain
public when explicitly selected or included in the configured public projection;
required temporal reads do not publish fields excluded by the request. Physical
`min_zoom` and `max_zoom` are always private, using ASCII case folding and physical
lineage, including renamed aliases.

## Resource and performance boundaries

The executor uses 256-row ordered keyset chunks. Request predicates
are limited to 512 identities and 128 combined spatial bounds. Variable row
payloads and buffered chunk payloads are each limited to 64 MiB. Payload overflow
fails explicitly. Raw geometry spatial predicates currently use an ordered scan;
no universal spatial-index pruning claim is made. The identity predicate can use
the proven unique index, subject to actual HANA planner evidence.

Exact DECIMAL properties preserve declared scale without conversion to float.
Configured SQL DECIMAL literals must fit the declared precision and scale before
binding as canonical decimal strings through an inner NVARCHAR(64) cast and an
outer declared DECIMAL cast. Actual string transport of signed 38-digit values and
scale-38 fractions was verified on HANA 2.00.088. Float predicates, unsupported collation profiles,
SMALLDECIMAL properties and unproven computed properties are unavailable.

## Evidence

Tile SQL dispatch preserves the separate legacy tile decoder. Raw WKB/WKT/MOS
queries use their own parameters; MOS `!BBOX!` expands to a literal bounds
predicate and does not receive native geometry parameters. Native tile queries
retain their native bbox bindings. Actual tile compatibility checks use BLOB
binary geometry and NCLOB text geometry, including extent clipping. The legacy
tile decoder does not extract geometry from VARBINARY columns; raw feature
queries support that storage independently. Custom tile SQL with unresolved
user parameter tokens is not admitted by the existing startup metadata probe.

`query_internal_test.go` covers pure metadata, strict decode, identifier quoting,
typed literal binding and temporal bounds. `query_snapshot_test.go` uses a mock
database/sql driver to cover executor order, cancellation, error chains, drift
guards and exact pagination. It does not simulate real HANA schema protection.

`query_live_contract_test.go` runs the real shared contract adapter only when
`RUN_HANA_TESTS=yes` and `HANA_CONNECTION_STRING` are provided. Without both, it
skips. Native malformed-wire construction is measured separately from raw WKB
query corruption. Native storage may reverse ring orientation, reject mixed
dimensional collections, or normalize an invalid child to an empty geometry;
these storage outcomes must not be reported as an original malformed-query pass.
The admitted native/raw profiles were verified by the live ordinary/custom,
dimensional, temporal and nullable matrices on HANA 2.00.088. Native storage
outcomes retain their separate classification.
