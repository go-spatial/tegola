# Read ADR-0010: Public CRS identity and immutable Part 2 projection

Status: accepted architecture; implemented public CRS registry. Runtime conformance evidence remains separately classified.

## Context

Part 2 publishes CRS URIs and supports output and bbox CRS selection. Numeric SRIDs are internal and may be synthetic or overridden by mutable projection registration. URI identity must not be inferred from a number.

## Decision

Use exact bounded local public identifiers and immutable owned converters. Standard descriptors initially cover CRS84, CRS84h, EPSG4326/4979, EPSG3857 and WGS84 UTM zones. CRS84 axes are longitude/latitude; EPSG4326/4979 axes latitude/longitude. Projected axes are easting/northing. Ellipsoidal height is WGS84 metres; never discard or manufacture it.

An application-defined CRS uses a registered `urn:uuid:` UUIDv8. Derive the first16 digest bytes from SHA256 over domain/version and length-prefixed effective horizontal definition, datum, dimension, vertical reference and ordered axis names/units; set UUID version8 and RFC variant bits. Retain the full digest and reject unequal descriptors sharing an identifier. Internal SRID is excluded. Public detached descriptor metadata defines the UUID's meaning locally; no remote lookup or request-driven registration occurs.

Global resolution handles shipped identifiers only. Application identifiers resolve through immutable collection instances. Source identity requires complete effective definition and provider proof. Canonical converters are independent of mutable projection registration; custom profiles are admitted only when an owned bidirectional converter proves the actual effective definition. Unsupported custom sources preserve Core and have no Part 2 capability claim. Current XY collections may publish admitted custom horizontal definitions through application CRS identifiers. This does not extend XYZ/mixed admission: those profiles require the separately proven height-preserving converters and vertical reference described in [Dimensional feature queries](ADR-0005-dimensional-feature-query.md).

Each XY collection publishes CRS84, EPSG4326/3857, and its proven actual source UTM/storage target. XYZ/mixed publishes CRS84h, EPSG4979/application3857-height, and its proven actual source target. Shipped URI resolution recognizes all WGS84 UTM zones, but does not publish far-zone targets for every collection. Uniform storage identity is included in that list. Genuinely mixed dimensional storage has no single uniform storage claim. Mixed default uses CRS84h for every page, with XY child height missing/unknown, never zero. This is documented application policy inferred from optional third ordinates. Coordinate families and nonempty child order remain intact.

Absent bbox-crs retains Core default by four/six coordinate arity. Explicit identifiers must belong to the collection catalog and match bbox dimension. Output requests incompatible with preservation of source height return400. All successful feature representations include Content-Crs `<URI>`, including defaults and individual features.

A bounded neutral BoundsCRSDefinition identifies the frozen query target independently of numeric mapping. It is authoritative: numeric equality cannot bypass a transform or enable source-space index pruning when definitions differ. Exact intersection stays in the original query frame before paging/count. Provider admission validates target definition/dimension before I/O. Existing tile and nil-definition Core behavior remain compatible.

## Consequences and evidence

Registry tests prove exact URI matching, axis permutation, deterministic content identity, detached metadata, canonical numeric fixtures and concurrency. Provider/service/HTTP integration and official Part2 ATS/ETS plus supplemental custom/mixed tests are separate requirements for CRS acceptance. No conformance or certification claim follows from this ADR alone.

Sources: [OGC API Features Part2](https://docs.ogc.org/is/18-058r1/18-058r1.html) sections6.2–6.3; [RFC9562](https://www.rfc-editor.org/rfc/rfc9562.html) sections5.8/6.5 and AppendixB.2. ADR-0005 supplies the height-preserving profile.
