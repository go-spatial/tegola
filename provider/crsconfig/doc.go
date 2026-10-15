// Package crsconfig implements the CRS part of the shared provider contract
// (see docs/crs.md).
//
// Every standard provider resolves a layer CRS through the same precedence
// order:
//
//  1. layer crs_defn
//  2. layer srid
//  3. provider crs_defn
//  4. provider srid (if explicitly configured)
//  5. source-derived SRID (database metadata, geometry header or the MOS
//     system-info Projection blob)
//  6. provider default (usually 3857)
//
// Main entry point:
//
//   - ResolveLayer: validates the layer configuration (srid / crs_defn) and
//     returns the resolved CRS together with a flag telling whether the
//     value was configured explicitly (which disables source-derived and
//     provider-default fallbacks).
//
// A custom PROJ.4 definition (crs_defn, or a Projection decoded from MOS
// system info) is validated through vendored proj and registered under a
// synthetic internal SRID. Synthetic SRIDs occupy the range >= 340000001
// and must never collide with real EPSG codes; they are meaningless outside
// the current process and cannot be persisted.
//
// Feature reads and mutations use NewFeatureProjection to own their converter
// independently of that registry. In addition to canonical WGS84 profiles, it
// admits custom etmerc definitions with explicit horizontal datum semantics.
// Their CanonicalSRID is zero; horizontal admission never proves a vertical
// datum, public EPSG identity, or safe ellipsoidal-height preservation.
package crsconfig
