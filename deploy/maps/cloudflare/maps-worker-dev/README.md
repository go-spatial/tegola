# Development maps Worker

This Worker binds the private `maps-dev` R2 bucket to `maps-dev.meistercrm.com`.
It uses the official PMTiles R2 range-read pattern: clients receive only
TileJSON and visible Z/X/Y tiles, while the raw archives stay private.

Bootstrap once after authenticating Wrangler:

```sh
cd deploy/maps/cloudflare/maps-worker-dev
npx wrangler r2 bucket create maps-dev --location=weur
npm ci
npx wrangler deploy
```

The configured custom domain is part of `wrangler.jsonc`. Keep the bucket
private: all browser access must go through the Worker so immutable tiles and
assets receive the intended CORS and cache policy.
