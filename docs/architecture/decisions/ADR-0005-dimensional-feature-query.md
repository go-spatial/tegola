# ADR-0005: Dimensional raw feature queries

Status: accepted on 2026-10-01 after independent architecture and provider feasibility review. Architecture acceptance is separate from runtime and conformance verification.

## Context

[ADR-0001](ADR-0001-feature-service-boundaries.md) separates raw providers, FeatureService and HTTP. [ADR-0002](ADR-0002-absent-feature-geometry.md) defines absent spatial geometry. [ADR-0003](ADR-0003-temporal-metadata-and-resolved-collections.md) establishes immutable metadata and constructor validation. Runtime/configuration wiring is a separate decision from this dimensional contract.

The existing query carries horizontal extents only. `geom` provides XYZ geometry types, but current WKB/WKT decoding, raw predicates, projection traversal and GeoJSON serialization handle XY only. GeoPackage XYZ envelopes do not establish lossless XYZ body decoding. MOS stores XY pairs. A six-coordinate query cannot be implemented by silently discarding height or supplying zero.

## Proposed decision

### Additive neutral API

Keep existing horizontal query fields and add:

```go
// Extent3D is [minX, minY, minZ, maxX, maxY, maxZ].
type Extent3D [6]float64

// Add to FeatureQuery:
// Bounds3D []Extent3D
// BoundsVerticalCRS string

type CoordinateDimension uint8
const (
    DimensionUnknown CoordinateDimension = iota
    DimensionXY
    DimensionXYZ
    DimensionMixedXYXYZ
)

type SpatialMetadata struct {
    Dimension CoordinateDimension
    VerticalCRS string
}

type SpatialLayerInfo interface {
    SpatialMetadata() (SpatialMetadata, error)
}
```

`Bounds` and `Bounds3D` are mutually exclusive; an empty pair imposes no spatial restriction. Existing `BoundsSRID` identifies horizontal axes for either representation. Require a positive horizontal SRID with either representation, and zero without both. Require `BoundsVerticalCRS` exactly when `Bounds3D` is present. Validate all six coordinates as finite and all three minima as no greater than maxima; equality selects an inclusive degenerate box. Structural failures return `InvalidFeatureQueryError`. Providers validate capability before I/O and return errors matching `ErrUnsupported` for unsupported operations.

Members of either spatial slice form a union, combined with IDs and temporal constraints by AND. Split a wrapping geographic box into ordinary boxes while retaining its vertical interval. Bound the union to the existing provider limit of 128 boxes. Retain input ownership, deduplication before paging/counting, synchronous callbacks and cancellation/error contracts.

Metadata is an immutable value snapshot. Unknown dimension is not proof of XYZ support; M is a measure, never Z. A declared mixed profile permits childwise XY/XYZ collections. Native GeoPackage `geometry_columns.z/m` supplies schema evidence, checked against encountered bodies; optional Z does not justify inventing Z in XY rows. Raw formats require explicit dimensional configuration and validation. Sampling alone cannot prove all-row dimensional eligibility.

### Initial vertical reference and transforms

Freeze GPKG source configuration: optional `spatial_dimension` is `xy`, `xyz` or `mixed_xy_xyz`; optional `vertical_crs` is the canonical CRS84h identifier below. Native GeoPackage infers dimension from valid geometry_columns.z values 0/1/2 when no override exists; explicit overrides must agree. Validate m as 0/1/2 independently; feature profiles declaring actual measure ordinates are unsupported initially. Raw WKB/WKT/MOS default to XY for compatibility; raw XYZ/mixed requires explicit spatial_dimension. XY forbids vertical_crs; XYZ/mixed requires it. Unknown/unsupported dimensional eligibility is retained as a feature capability error, without breaking legacy tile registration; syntactically invalid explicit configuration fails registration. Explicit XYZ/mixed without the required vertical key and whitespace-only explicit references are invalid configuration. Native-inferred XYZ without new dimensional keys retains a capability error when vertical metadata is absent. Every encountered raw body is checked against the declared dimension, so raw XY never silently decodes Z as XY.

Initial height-preserving horizontal transformations admit only the reviewed canonical WGS84 profiles EPSG:4326, EPSG:3857 and EPSG:32601–32660/32701–32760, in both query and source roles. XYZ/mixed source admission retains effective configuration/schema provenance and rejects custom/synthetic horizontal definitions, including inherited `crs_defn`, for this height-preserving profile. Numeric overrides explicitly declare the canonical CRS under the established configuration precedence; verify resolved source metadata accordingly. Custom or other source/query horizontal definitions are unsupported for XYZ until separately reviewed; successful 2D converter construction is insufficient. This restriction applies only to XYZ/mixed height preservation. Current XY publication supports admitted custom horizontal definitions, including the bounded etmerc/datum profile in `provider/crsconfig/custom_projection.go`; its zero-height horizontal datum conversion does not establish vertical-reference equivalence. See the [current CRS contract](../../crs.md).

The geometry evaluation frame is the query CRS: transformed vertices define straight segments and the declared planar polygon surface there, matching the existing vertex-transform model. Nonlinear horizontal projection can turn a source-planar polygon into a nonplanar vertex set; reject such transformed surfaces with ErrUnsupported. Source planarity alone does not establish query-frame planarity. This representation model is an application decision, not a claim of analytic preservation of curved transformed segments.

Planarity is tested by robust determinant arithmetic over the actual finite binary floating point coordinates, with exact rational fallback; do not flatten nearly planar coordinates through an undocumented epsilon. Planarity and intersection use the same declared surface.

Dependency publication amendment: the standalone proj fork owns its active source; a vendor-only API would break published dependency consumption. Instead introduce `crsconfig.NewHeightProjection(srid uint64) (*HeightProjection, error)`, with Definition, Forward and Inverse methods. Construct an owned immutable converter directly from fixed reviewed canonical definitions through exported proj/support and proj/core APIs. No mutable projection-registry lookup participates in admission, filtering or XYZ output. Registry overrides therefore cannot alter the canonical XYZ profile. Hold private converter access under an instance mutex; handle degrees/radians, units, finite inputs/outputs and projection hooks explicitly. WGS84-only definitions require no datum conversion. Source admission must reject effective custom/synthetic definitions, not infer provenance from a successful constructor alone. Retain the canonical source adapter through filtering and serialization; construct the canonical query adapter before processing. Existing XY and tile conversions retain their established behavior. Test concurrent registry override isolation. No dependency source, version, checksum or vendor modification is required.

Native envelopes may legally be NONE. Interpret envelope flags rather than ordinate count: XYZ and XYM both have six envelope values but different meanings. An omitted envelope does not imply XY. Check actual nonempty body dimensions against z/m schema; m=2 permits but does not prove measure ordinates. Reject encountered unsupported M/ZM bodies; optional measures alone do not reject otherwise admitted XY/XYZ rows.

The initial vertical profile is CRS84h: WGS84 ellipsoidal height in metres. Use the canonical identifier `http://www.opengis.net/def/crs/OGC/0/CRS84h` for the vertical-reference binding, while retaining the existing horizontal-SRID contract. This binding does not declare arbitrary numeric horizontal SRIDs to be three-dimensional CRSs. Other vertical references, units or height transformations are unsupported in this initial profile.

XYZ source metadata must explicitly bind to this reference. Never infer ellipsoidal metres from a third ordinate, a horizontal EPSG identifier or an XYZ envelope. XY-only sources declare no vertical reference; mixed sources bind their XYZ members to CRS84h.

An XY coordinate transformation may preserve Z numerically only when it changes projection/axis representation without a horizontal datum shift that affects height. Providers must establish this capability explicitly. The existing 2D projection engine and successful engine construction do not prove that condition. Datum-shift pipelines, including applicable `towgs84` transformations, are unsupported for XYZ until a reviewed true 3D conversion handles height. Do not apply XY quantization or unit scaling to Z. A true 3D conversion must transform all ordinates consistently and declare its vertical semantics.

### Geometry and absence semantics

Preserve native XYZ coordinates in raw features and GeoJSON. A horizontal query against XYZ evaluates the XY projection of the geometry while validating and retaining Z in the returned payload. A three-dimensional query evaluates a closed box:

- XYZ points use inclusive membership on all axes.
- XYZ lines use segment/box intersection with a shared segment parameter interval across X, Y and Z. Separate horizontal intersection and global Z-range overlap are insufficient.
- Planar XYZ polygons represent surfaces with holes. Intersect the box with the polygon's plane and test that intersection against the polygon surface, including hole boundaries. Constant-height polygons may reuse the exact horizontal predicate plus height membership. Vertical planes and degenerate boxes require explicit handling. Nonplanar polygon surfaces are unsupported; do not choose an arbitrary triangulation or compare only envelopes.
- Multi-geometries and collections use childwise union. Validate all coordinates, shapes, dimensional declarations and children before accepting any positive child. A matching sibling must not hide malformed or unsupported data.
- Nil and wholly decoded empty geometry retain ADR-0002 absence semantics and match a valid box. Malformed input, nonfinite coordinates and malformed partial containers remain errors. An empty child does not make a nonempty collection absent.

For **nonempty XY geometry under a six-coordinate box**, propose an explicit application policy: its vertical dimension is unconstrained, so evaluate its exact horizontal intersection and preserve its XY payload. This is an architectural inference requiring independent review, not a claim that the specification mandates this interpretation. It does not fabricate a height or classify nonempty XY geometry as wholly absent. In heterogeneous collections, apply this policy to each XY child and true 3D intersection to each XYZ child; validate the entire collection first. If review rejects this policy, replace it explicitly before implementation rather than silently treating XY as zero height or rejecting selected rows after publishing a contradictory capability.

### Codec and provider boundaries

Introduce raw-only dimensional decoding; leave existing tile decoding/filtering and tile projection behavior intact. Initially support native GeoPackage bodies, ISO-WKB Z types 1001–1007 and WKT Z for supported geometry families, including nested collections. Preserve count, depth, byte-order, input-length and malformed-input protections; define strict trailing-data and empty-point policies. Reject unsupported M/ZM or EWKB variants explicitly unless separately specified and reviewed. Wire polygon rings require at least four positions and identical first/last coordinates, including Z. Mixed XY/XYZ collections are an explicit application extension; do not claim complete ISO-WKT conformance. GeoPackage dimensional schema/header/body disagreement must fail closed under the raw profile.

MOS remains XY. Under the proposed XY policy it can satisfy a 3D query through exact horizontal intersection with unconstrained vertical dimension; it must never advertise lossless XYZ storage or return fabricated height.

Keep SQL coarse filtering horizontal where appropriate. Project each 3D box to its XY extent for candidate selection, preserve the absence/indexless branches, and apply the exact dimensional predicate before offset, limit and counts. XYZ envelopes and RTree candidates are not final vertical predicates. Retain bound SQL values, stable identity, scoped cursor cleanup, context checks and snapshot behavior.

## Required implementation gaps

1. Add query validation and immutable optional spatial metadata without changing `LayerInfo`, `Tiler` or `Feature` representation.
2. Implement strict raw dimensional WKB/WKT decoding and native GeoPackage dispatch; isolate it from tile decoding. Add immutable source dimensional/vertical configuration and schema checks.
3. Add pure exact dimensional validation/intersection, planar-surface handling with holes and robust boundary arithmetic; current horizontal helper rejects Z types.
4. Add raw-only coordinate traversal that preserves Z for admitted projection-only transformations and rejects unsafe datum operations.
5. Extend FeatureService catalog, six-coordinate parsing and recursive GeoJSON serialization while preserving dimensional coordinates and ownership.
6. Extend reusable provider contract cases and independently frozen acceptance oracles. Update configuration/runtime wiring under its separate decision.

This architectural decision alone does not certify any codec/provider as XYZ-capable. New API names and metadata configuration require independent compatibility review before acceptance.

## Required evidence before completion

- Four/six-coordinate structural validation, mutually exclusive bounds, vertical bindings, finite values, reversed and degenerate axes, antimeridian unions and ownership.
- ISO-WKB/native/WKT Z point, line, planar polygon with holes, multi-geometries and nested mixed collections; exact ordinates through decode/query/GeoJSON; malformed counts, truncation, depth, trailing input, metadata disagreement and unsupported M/ZM/EWKB profiles.
- Segment correlation counterexamples where independent XY/Z ranges overlap but the segment misses the box; vertical and tilted planar polygons, holes, touching faces/edges, degenerate boxes and nonplanar rejection.
- XY policy and true absence tested separately; empty children must not override nonempty siblings; invalid child after matching child must fail.
- Height-preserving projection evidence and datum-shift rejection, with independent expected XYZ values and no production roundtrip oracle.
- Exact predicates before paging/counting, SQL pruning claims scoped honestly, cancellation, callback error chains, cursor cleanup, repeatability and concurrent ownership.
- Existing 2D raw and tile behavior retained, including strict malformed geometry and legacy tile exclusion policies.

Independent reviewers must examine geometry/CRS semantics, provider feasibility and service compatibility when accepting or amending this decision. Architecture acceptance alone does not establish runtime or Core conformance.
