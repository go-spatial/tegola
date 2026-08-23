<!-- Staging and development map release guidance explains immutable R2 publication, Worker delivery, environment-specific manifests, validation, promotion, rollback, and retired local runtime dependencies for operators safely. -->
# Self-Hosted Proposal Map Data

Proposal maps use versioned R2 manifests. The release workflows publish the
grouped basemap, country packs, glyphs and sprites under immutable release
keys. Browser traffic goes through the Maps Worker; it must not use the Railway
Router, `/map-assets`, or `/tegola`.

## Environment targets

- Development: `https://maps-dev.meistercrm.com/v1/current.json`, served by
  `maps-worker-dev` from the private `maps-dev` bucket.
- Staging: `https://maps-staging.meistercrm.com/v1/current.json`, served by
  `maps-worker-staging` from the private `maps-staging` bucket.

Staging is hydrated by copying a verified immutable development release into
`maps-staging`, rewriting its manifest URLs for the staging host, and only
then publishing `v1/current.json`. It does not rebuild the source data.

The local commands below are retained for troubleshooting and break-glass
fallback only. They do not build or publish the R2 pilot release.

This directory contains the reproducible local/prod setup for the free map stack:

- `east-africa.pmtiles` is generated from Protomaps/OpenStreetMap and served as a static file.
- Protomaps font and sprite assets are copied into the same static asset directory.
- OSM protected-area polygons are imported into PostGIS and served by Tegola as `protected_areas`.

Generated map data is ignored by git.

Development release operations are split by cost: `Maps data build/upload
(development)` performs the one-off source downloads and archive build, while
`Maps release promotion (development)` verifies a complete immutable R2
release and only updates `v1/current.json`. Routine promotion therefore does
not redownload Protomaps or Geofabrik data. Staging copies a verified
development release instead of rebuilding it.

## Legacy local fallback: basemap assets

```sh
cd /c/Users/alexb/code/tegola
./deploy/maps/build-east-africa-pmtiles.sh
```

The generated files are served by the legacy local `map-assets` container at:

```text
http://localhost:8088/map-assets/east-africa.pmtiles
http://localhost:8088/map-assets/protomaps-assets/fonts/{fontstack}/{range}.pbf
http://localhost:8088/map-assets/protomaps-assets/sprites/v4/light.json
http://localhost:8088/map-assets/protomaps-assets/sprites/v4/light.png
```

Use the shared local Docker stack to serve the generated files:

```powershell
cd C:\Users\alexb\code\local-dev
docker compose --profile all up map-assets
```

## Legacy local fallback: Tegola protected areas

Start the local map database and run the Compose-networked importer:

```powershell
cd C:\Users\alexb\code\local-dev
docker compose up -d maps-postgis
docker compose --profile maps-import run --rm maps-importer
```

For faster local iteration you can limit the import to selected countries:

```powershell
$env:OSM_COUNTRIES = "tanzania kenya"
docker compose --profile maps-import run --rm maps-importer
Remove-Item Env:\OSM_COUNTRIES
```

By default the script drops osm2pgsql middle tables after the final country to keep the
database smaller. Set `OSM2PGSQL_DROP_MIDDLE_TABLES=false` if you need to keep them.

Then start Tegola through local Compose:

```powershell
docker compose --profile all up tegola
```

The `/tegola` route and Router-backed map volume are legacy fallback paths in
development. Locally there is no `global-router`; frontends can call the direct
services when deliberately testing the fallback:

```text
http://localhost:8088/map-assets/east-africa.pmtiles
http://localhost:9090/capabilities
http://localhost:9090/capabilities/protected_areas.json
http://localhost:9090/maps/protected_areas/5/19/16.pbf
```
