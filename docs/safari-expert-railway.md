# Safari Expert Railway Deployment

> **Development map delivery:** the active pilot is static PMTiles served by
> `maps-worker-dev` from private R2. Frontends read
> `https://maps-dev.meistercrm.com/v1/current.json`; they do not use the
> Railway Router, `/map-assets`, or `/tegola` for normal proposal maps.

The Railway instructions below are retained for the legacy Tegola fallback and
break-glass rollback only. Do not add Southern or East pilot archives to the
Router volume. To roll back the pilot, intentionally unset
`VITE_MAP_MANIFEST_URL` in the development frontend build before redeploying.

This repo tracks upstream Tegola, so Safari Expert deployment files are kept in separate `railway.*` and `deploy/railway/*` files to reduce fork-sync conflicts.

## Runtime

Railway should build with `Dockerfile.railway` through `railway.toml`. The container starts Tegola with:

```sh
/opt/tegola serve --config "$TEGOLA_CONFIG" --port "[::]:$PORT"
```

Set these Tegola service variables in Railway:

- `PORT=8080`
- `TEGOLA_BIND_HOST=::`
- `TEGOLA_CONFIG_URL=https://.../config.toml` or `TEGOLA_CONFIG=/opt/tegola_config/config.toml`
- `POSTGIS_URI=postgres://...` for the protected-area PostGIS provider

If using a mounted config file, mount it at `/opt/tegola_config/config.toml` or set `TEGOLA_CONFIG` to the mounted path.

For the legacy self-hosted map fallback, the Tegola config exposes the
`protected_areas` map from `deploy/maps/tegola.protected_areas.toml`. The pilot
basemap and country overlays are no longer served from this service.

## Legacy Router fallback

The old browser path is the global router, not the Tegola service domain:

```text
https://dev.meistercrm.com/tegola
```

Set the `global-router` Railway variable:

```text
TEGOLA_URL=http://tegola.railway.internal:8080
```

The router strips `/tegola/` before proxying. It also maps `/tegola/maps/{mapName}` to Tegola's native `/capabilities/{mapName}.json` endpoint so frontend clients can use the POC TileJSON path consistently.

The router may still serve self-hosted basemap assets for rollback diagnostics:

```text
https://dev.meistercrm.com/map-assets
```

If the fallback is explicitly enabled, mount the generated map asset directory
into `global-router` and set:

```text
MAP_ASSETS_ROOT=/opt/map-assets
```

## CI Deploy

`.github/workflows/railway-deploy.yml` deploys after the upstream `On push` workflow succeeds on `main`, and can also be triggered manually. It skips safely when Railway secrets are missing.

Required GitHub secrets:

- `RAILWAY_TOKEN`
- `RAILWAY_PROJECT_ID`
- `RAILWAY_TEGOLA_SERVICE_ID`

Optional GitHub variable:

- `RAILWAY_ENVIRONMENT`, defaulting to `development`

## Legacy fallback smoke checks

After deploying Tegola and global-router, these should pass:

```sh
curl -i https://dev.meistercrm.com/tegola/capabilities
curl -i https://dev.meistercrm.com/tegola/maps/protected_areas
curl -i https://dev.meistercrm.com/tegola/maps/protected_areas/5/19/16.pbf
curl -I -H 'Range: bytes=0-1023' https://dev.meistercrm.com/map-assets/east-africa.pmtiles
```
