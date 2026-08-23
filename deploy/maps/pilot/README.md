# Mainland Africa map-release pilot

This is a dev-only static-map pilot. Releases no longer rebuild, read, or
depend on `/map-assets`, `/tegola`, or the Railway Router. The legacy stack is
kept only as an explicit rollback path while the static release is adopted.

## Coverage

- Protected-area packs: `BW`, `KE`, `LS`, `MW`, `MZ`, `NA`, `RW`, `SZ`, `TZ`,
  `UG`, `ZA`, `ZM`, and `ZW`.
- Basemap-only context: `BI`.
- Deferred: `ET`, `MG`, `MU`, `RE`, and `SC`.

`ZA` is South Africa. It is a required country pack and its build is gated on
Kruger being present.

## Locked inputs

`release-inputs.lock.json` is intentionally committed. It pins:

- the Protomaps source key, release version, content length, ETag, and
  published BLAKE3/MD5 metadata;
- Natural Earth country polygons and SHA-256;
- the exact Protomaps glyph/sprite commit and SHA-256 archive;
- each Geofabrik input URL/MD5, including the `swaziland` extract for `SZ`.

Do not update it during a release run. A changed Geofabrik `-latest` file makes
the build fail; update the lock in a reviewed change, then run the release with
that exact commit.

## Build locally

The build uses the official PMTiles `extract --region` workflow. It makes a
GeoJSON feature collection from *buffered* country polygons first, rather than
using a large rectangular bounding box.

```sh
bash deploy/maps/pilot/build-release.sh \
  --release-id 20260719-20260718-abcdef0
```

The build output contains only immutable release objects:

```text
releases/<release>/basemap/safari-mainland-pilot.pmtiles
releases/<release>/protected-areas/ZA.pmtiles
releases/<release>/assets/...
releases/<release>/manifest.json
releases/<release>/release.lock.json
```

Each country is imported into a new temporary PostGIS schema, exported with
GDAL's PMTiles driver at zooms 4–12, checked for a non-empty polygon layer and
the listed known park, and written with two layers:

- `protected_areas`
- `protected_area_labels` (one `ST_PointOnSurface` label point per named area)
- `tourism_areas`, `tourism_pois`
- `heritage_areas`, `heritage_pois`
- `outdoor_routes` (named hiking, walking, and mountain-bike route relations)
- `water_sport_pois` (named dive, snorkelling, surfing, kayaking, and rafting sites)

The exporter performs name cleanup and identity de-duplication before writing
the archive. Every pack receives quality metadata including published/dropped
counts, route and heritage counts, de-duplication drops, bounds, and checksums.
Run `performance-report.sh` against a local release to record archive-size and
feature-density metrics. No R2 upload or manifest activation is part of a local
pilot run.

The basemap is built at zooms 0–12. MapLibre overzooms above that range.

## Publishing and rollback

The expensive build is intentionally a one-off data operation. Run the manual
`Maps data build/upload (development)` workflow only when creating a new
immutable release; it downloads the pinned inputs, verifies every PMTiles
archive, and uploads the release objects to R2 without overwriting existing
keys. Normal development releases use the fast `Maps release promotion
(development)` workflow: provide an already-uploaded release ID, let it verify
the manifest, lock checksums, archive objects, and representative Worker
tiles, then replace `v1/current.json` last. `maps-dev` stays private;
`maps-worker-dev` is the only browser-serving path.

To roll back map data, copy a previous immutable
`releases/<release>/manifest.json` to `v1/current.json`. To roll back the
entire pilot, unset `VITE_MAP_MANIFEST_URL` from the development frontend build
and redeploy it; this intentionally re-enables the retained legacy fallback.

## Deprecated legacy East fallback

`rebuild-legacy-east-protected-areas.sh` is a break-glass rollback utility and
is not run by map releases. It imports Tanzania, Kenya, Uganda, Rwanda, and
Burundi into a staging schema, requires Bwindi plus one of Murchison Falls,
Queen Elizabeth, or Kidepo, then swaps `gis.protected_areas` in one
transaction. It never places Southern pilot archives on Railway.
