// This Worker serves versioned private R2 map releases as manifests, vector tiles, glyphs, and sprites through custom domains with caching and controlled browser access safely.
import {
  Compression,
  EtagMismatch,
  PMTiles,
  ResolvedValueCache,
  TileType,
  tileTypeExt,
  type RangeResponse,
  type Source,
} from "pmtiles";

interface Env {
  ALLOWED_ORIGINS?: string;
  BUCKET: R2Bucket;
  IMMUTABLE_CACHE_CONTROL?: string;
  MANIFEST_CACHE_CONTROL?: string;
  MAP_UPLOAD_TOKEN?: string;
  MAP_UPLOAD_PREFIX?: string;
}

class KeyNotFoundError extends Error {}

const CACHE = new ResolvedValueCache(25, undefined, nativeDecompress);
// `caches.default` is a Cloudflare runtime extension; its published
// CacheStorage declaration intentionally omits it.
const edgeCache = (): Cache =>
  (caches as unknown as { default: Cache }).default;
const immutableCacheControl = (env: Env): string =>
  env.IMMUTABLE_CACHE_CONTROL ?? "public, max-age=31536000, immutable";
const manifestCacheControl = (env: Env): string =>
  env.MANIFEST_CACHE_CONTROL ?? "public, max-age=60";

async function nativeDecompress(
  bytes: ArrayBuffer,
  compression: Compression,
): Promise<ArrayBuffer> {
  if (compression === Compression.None || compression === Compression.Unknown) {
    return bytes;
  }
  if (compression === Compression.Gzip) {
    const stream = new Response(bytes).body?.pipeThrough(
      new DecompressionStream("gzip"),
    );
    return new Response(stream).arrayBuffer();
  }
  throw new Error("unsupported PMTiles compression");
}

class R2Source implements Source {
  constructor(
    private readonly env: Env,
    private readonly archiveKey: string,
  ) {}

  getKey(): string {
    return this.archiveKey;
  }

  async getBytes(
    offset: number,
    length: number,
    _signal?: AbortSignal,
    etag?: string,
  ): Promise<RangeResponse> {
    const object = await this.env.BUCKET.get(this.archiveKey, {
      onlyIf: etag ? { etagMatches: etag } : undefined,
      range: { length, offset },
    });
    if (!object) {
      throw new KeyNotFoundError("archive not found");
    }
    if (!object.body) {
      throw new EtagMismatch();
    }
    return {
      cacheControl: object.httpMetadata?.cacheControl,
      data: await object.arrayBuffer(),
      etag: object.etag,
      expires: object.httpMetadata?.cacheExpiry?.toISOString(),
    };
  }
}

const decodePath = (pathname: string): string[] => {
  const segments = pathname.split("/").filter(Boolean).map(decodeURIComponent);
  if (
    segments.length === 0 ||
    segments.some(
      (segment) =>
        segment.length === 0 || segment === "." || segment === ".." || segment.includes("\\"),
    )
  ) {
    throw new Error("invalid path");
  }
  return segments;
};

const allowedOrigin = (request: Request, env: Env): string | null => {
  const configured = env.ALLOWED_ORIGINS?.split(",").map((origin) => origin.trim()) ?? [];
  if (configured.includes("*")) return "*";
  const origin = request.headers.get("Origin");
  return origin && configured.includes(origin) ? origin : null;
};

const withCors = (response: Response, origin: string | null): Response => {
  const headers = new Headers(response.headers);
  if (origin) {
    headers.set("Access-Control-Allow-Origin", origin);
    headers.set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS");
    headers.set("Access-Control-Allow-Headers", "Range");
    if (origin !== "*") headers.set("Vary", "Origin");
  }
  return new Response(response.body, { headers, status: response.status });
};

const uploadAuthorized = (request: Request, env: Env): boolean => {
  const token = env.MAP_UPLOAD_TOKEN?.trim();
  return Boolean(token && request.headers.get("Authorization") === `Bearer ${token}`);
};

const uploadKey = (request: Request, env: Env): string | null => {
  const key = new URL(request.url).searchParams.get("key");
  const prefix = env.MAP_UPLOAD_PREFIX?.trim();
  return key && prefix && key.startsWith(prefix) ? key : null;
};

const handleTemporaryMultipartUpload = async (
  request: Request,
  env: Env,
  segments: string[],
): Promise<Response> => {
  if (!uploadAuthorized(request, env)) return new Response("not found", { status: 404 });
  const key = uploadKey(request, env);
  if (!key) return new Response("invalid upload key", { status: 400 });

  if (request.method === "POST" && segments[1] === "init") {
    const upload = await env.BUCKET.createMultipartUpload(key, {
      httpMetadata: { contentType: request.headers.get("Content-Type") ?? "application/octet-stream" },
    });
    return Response.json({ uploadId: upload.uploadId, key });
  }

  const uploadId = segments[1];
  if (!uploadId) return new Response("invalid upload id", { status: 400 });
  const upload = env.BUCKET.resumeMultipartUpload(key, uploadId);

  if (request.method === "DELETE" && segments[2] === "abort") {
    await upload.abort();
    return new Response(null, { status: 204 });
  }

  if (request.method === "PUT" && segments[2] === "part") {
    const partNumber = Number(segments[3]);
    if (!Number.isInteger(partNumber) || partNumber < 1 || partNumber > 10000 || !request.body) {
      return new Response("invalid part", { status: 400 });
    }
    const part = await upload.uploadPart(partNumber, request.body);
    return Response.json({ partNumber: part.partNumber, etag: part.etag });
  }

  if (request.method === "POST" && segments[2] === "complete") {
    const payload = (await request.json()) as { parts?: Array<{ partNumber: number; etag: string }> };
    if (!Array.isArray(payload.parts) || payload.parts.length === 0) {
      return new Response("invalid parts", { status: 400 });
    }
    const completed = await upload.complete(payload.parts);
    return Response.json({ etag: completed.etag, key });
  }

  return new Response("not found", { status: 404 });
};

const contentTypeForPath = (path: string): string | undefined => {
  if (path.endsWith(".pbf") || path.endsWith(".mvt")) {
    return "application/vnd.mapbox-vector-tile";
  }
  if (path.endsWith(".json")) return "application/json";
  if (path.endsWith(".png")) return "image/png";
  if (path.endsWith(".webp")) return "image/webp";
  return undefined;
};

const cacheKeyFor = (url: URL): Request => new Request(url.toString());

const markCache = (response: Response, state: "hit" | "miss"): Response => {
  const headers = new Headers(response.headers);
  headers.set("X-Map-Edge-Cache", state);
  return new Response(response.body, { headers, status: response.status });
};

async function cacheResponse(
  request: Request,
  url: URL,
  ctx: ExecutionContext,
  headers: Headers,
  body: BodyInit | null,
  status = 200,
): Promise<Response> {
  const response = new Response(request.method === "HEAD" ? null : body, {
    headers,
    status,
  });
  if (request.method === "GET" && status >= 200 && status < 300) {
    ctx.waitUntil(edgeCache().put(cacheKeyFor(url), response.clone()));
  }
  return response;
}

async function serveStaticObject(
  request: Request,
  url: URL,
  env: Env,
  ctx: ExecutionContext,
  key: string,
  cacheControl: string,
  useEdgeCache = true,
): Promise<Response> {
  if (useEdgeCache) {
    const cached = await edgeCache().match(cacheKeyFor(url));
    if (cached) return markCache(cached, "hit");
  }

  const object = await env.BUCKET.get(key);
  if (!object) return new Response("not found", { status: 404 });

  const headers = new Headers();
  object.writeHttpMetadata(headers);
  headers.set("Cache-Control", cacheControl);
  headers.set("ETag", object.httpEtag);
  headers.set("X-Map-Edge-Cache", "miss");
  headers.set("X-Map-Asset-Source", "r2");
  if (!headers.get("Content-Type")) {
    const contentType = contentTypeForPath(key);
    if (contentType) headers.set("Content-Type", contentType);
  }
  if (!useEdgeCache) {
    return new Response(request.method === "HEAD" ? null : object.body, { headers, status: 200 });
  }
  return cacheResponse(request, url, ctx, headers, object.body);
}

type TileRequest =
  | { archiveName: string; extension: string; tile: [number, number, number] }
  | { archiveName: string; extension: "json"; tile: null };

const parseTileRequest = (segments: string[]): TileRequest | null => {
  const last = segments.at(-1) ?? "";
  if (last.endsWith(".json") && !["manifest.json", "release.lock.json"].includes(last)) {
    const name = [...segments.slice(0, -1), last.slice(0, -5)].join("/");
    return name.startsWith("releases/") ? { archiveName: name, extension: "json", tile: null } : null;
  }
  if (segments.length < 4) return null;
  const [zText, xText, yWithExt] = segments.slice(-3);
  const match = /^(\d+)\.(mvt|pbf|png|jpg|webp|avif)$/.exec(yWithExt);
  if (!match || !/^\d+$/.test(zText) || !/^\d+$/.test(xText)) return null;
  const archiveName = segments.slice(0, -3).join("/");
  if (!archiveName.startsWith("releases/")) return null;
  return {
    archiveName,
    extension: match[2],
    tile: [Number(zText), Number(xText), Number(match[1])],
  };
};

async function servePmtiles(
  request: Request,
  url: URL,
  env: Env,
  ctx: ExecutionContext,
  parsed: TileRequest,
): Promise<Response> {
  const cached = await edgeCache().match(cacheKeyFor(url));
  if (cached) return markCache(cached, "hit");

  const headers = new Headers({
    "Cache-Control": immutableCacheControl(env),
    "X-Map-Edge-Cache": "miss",
    "X-Map-Asset-Source": "pmtiles-r2",
  });
  const archive = new PMTiles(new R2Source(env, `${parsed.archiveName}.pmtiles`), CACHE, nativeDecompress);
  try {
    if (!parsed.tile) {
      headers.set("Content-Type", "application/json");
      const tileJson = await archive.getTileJson(`${url.origin}/${parsed.archiveName}`);
      return cacheResponse(request, url, ctx, headers, JSON.stringify(tileJson));
    }

    const header = await archive.getHeader();
    const [z, x, y] = parsed.tile;
    if (z < header.minZoom || z > header.maxZoom) {
      return new Response("not found", { status: 404 });
    }
    const expectedType: Record<string, TileType> = {
      avif: TileType.Avif,
      jpg: TileType.Jpeg,
      mvt: TileType.Mvt,
      pbf: TileType.Mvt,
      png: TileType.Png,
      webp: TileType.Webp,
    };
    if (header.tileType !== expectedType[parsed.extension] && tileTypeExt(header.tileType) !== "") {
      return new Response(`wrong tile extension: .${parsed.extension}`, { status: 400 });
    }
    const tile = await archive.getZxy(z, x, y);
    if (!tile) return cacheResponse(request, url, ctx, headers, null, 204);

    const contentType: Partial<Record<TileType, string>> = {
      [TileType.Jpeg]: "image/jpeg",
      [TileType.Mvt]: "application/vnd.mapbox-vector-tile",
      [TileType.Png]: "image/png",
      [TileType.Webp]: "image/webp",
      [TileType.Avif]: "image/avif",
    };
    const resolvedContentType = contentType[header.tileType];
    if (resolvedContentType) headers.set("Content-Type", resolvedContentType);
    return cacheResponse(request, url, ctx, headers, tile.data);
  } catch (error) {
    if (error instanceof KeyNotFoundError) {
      return new Response("archive not found", { status: 404 });
    }
    throw error;
  }
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const origin = allowedOrigin(request, env);
    try {
      const url = new URL(request.url);
      const segments = decodePath(url.pathname);
      if (segments[0] === "__upload") {
        return await handleTemporaryMultipartUpload(request, env, segments);
      }
      if (request.method === "OPTIONS") {
        return withCors(new Response(null, { status: 204 }), origin);
      }
      if (request.method !== "GET" && request.method !== "HEAD") {
        return withCors(new Response("method not allowed", { status: 405 }), origin);
      }
      let response: Response;
      if (segments.join("/") === "v1/current.json") {
        response = await serveStaticObject(request, url, env, ctx, "v1/current.json", manifestCacheControl(env), false);
      } else if (segments[0] === "releases" && segments.includes("assets")) {
        response = await serveStaticObject(request, url, env, ctx, segments.join("/"), immutableCacheControl(env));
      } else if (segments[0] === "releases" && ["manifest.json", "release.lock.json"].includes(segments.at(-1) ?? "")) {
        response = await serveStaticObject(request, url, env, ctx, segments.join("/"), immutableCacheControl(env));
      } else {
        const tileRequest = parseTileRequest(segments);
        if (tileRequest) {
          response = await servePmtiles(request, url, env, ctx, tileRequest);
        } else {
          response = new Response("not found", { status: 404 });
        }
      }
      return withCors(response, origin);
    } catch (error) {
      console.error("maps worker request failed", error);
      return withCors(new Response("map delivery failed", { status: 500 }), origin);
    }
  },
};
