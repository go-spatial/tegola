// Package geometrycodec implements the geometry-format part of the shared
// provider contract (see docs/provider-contract.md).
//
// All standard data providers (mysql, gpkg, postgis, hana) decode feature
// geometries through this package so that per-format behaviour — WKB, WKT
// and the packed MOS ("MapplGIS") payload — stays consistent across
// backends.
//
// Configuration keys (provider and layer level):
//
//   - geometry_format: wkb, wkt or mos (providers may add their own native
//     values, e.g. mysql's auto/mysql/mariadb or gpkg's gpkg).
//   - mos_precision: decimal digits carried by MOS coordinates.
//   - mos_units: packed linear unit of MOS coordinates (mm, cm, dm, m, km).
//
// Main entry points:
//
//   - ResolveLayerGeometryFormat: merges the provider-level format with the
//     layer override and validates the value against a provider-supplied
//     set of accepted values.
//   - WarnAndResetMOSParams: warns about mos_precision / mos_units settings
//     that cannot take effect for the effective format and resets them.
//   - ValidateRawCustomSQL: rejects raw-format custom SQL that still uses a
//     native spatial predicate instead of !BBOX!.
//   - IsRawFormat / IsSystemInfoValue: format classification helpers.
//
// Invariants:
//
//   - MOS coordinates carry no CRS; the layer CRS comes from the provider
//     (srid / crs_defn / system-info Projection), never from the payload.
//   - The startup inspection window is InspectionSampleLimit rows wide for
//     every provider, so system-info auto-configuration behaves identically
//     regardless of backend.
package geometrycodec
