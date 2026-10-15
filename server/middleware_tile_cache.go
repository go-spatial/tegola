package server

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/internal/log"
)

// tileRenders deduplicates concurrent cache-miss renders of the same tile so a
// thundering herd of identical requests triggers a single render.
var tileRenders tileRenderGroup

// TileCacheHandler implements a request cache for tiles on requests when the URLs
// have a /:z/:x/:y scheme suffix (i.e. /osm/1/3/4.pbf)
func TileCacheHandler(a *atlas.Atlas, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error

		// check if a cache backend exists
		cacher := requestTileCache(r.Context(), a.GetCache())
		if editorTileCacheBypass(r) {
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
			return
		}
		if cacher == nil {
			// nope. move on
			next.ServeHTTP(w, r)
			return
		}

		// parse our URI into a cache key structure (remove any configured URIPrefix + "maps/" )
		key, err := cache.ParseKey(strings.TrimPrefix(r.URL.Path, path.Join(URIPrefix, "maps")))
		if err != nil {
			log.Errorf("cache middleware: ParseKey err: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		query := r.URL.Query()
		_, hasDirty := query[QueryKeyDirty]
		dirty := query.Get(QueryKeyDirty)
		forceRegenerate := hasDirty && (dirty == "" || dirty == "1" || strings.EqualFold(dirty, "true"))

		// The regenerating ?dirty variant ( ?dirty, ?dirty=1, ?dirty=true )
		// drives cache maintenance: it is gated like the ?tile= operations
		// (disabled by default, token + rate limit when enabled). Only that
		// variant is an operation; a falsy value (?dirty=0, ?dirty=false)
		// requests no regeneration, trips no gate and is served as an
		// ordinary cache query.
		if forceRegenerate {
			release, err := gateTileOperation(r, len(query) == 1)
			if err != nil {
				log.Debugf("cache middleware: dirty regeneration denied for %v: %v", r.URL.Path, err)
				writeTileOperationDenied(w, err)
				return
			}
			defer release()
		}

		// A dirty request is only cacheable when it is the sole query
		// parameter. Caching a response that depends on map query parameters
		// under the ordinary tile key would serve the wrong data later.
		if forceRegenerate && len(query) == 1 {
			// a regeneration is a cache mutation: serialize it against other
			// metatile updates and mark the metatile as updating so concurrent
			// renders know their result is stale.
			state, unlock, err := tileUpdateLocks.acquire(r.Context(), cacheMetatileLockKey(cacher, key))
			if err != nil {
				// the request was canceled while waiting for the metatile lock
				log.Debugf("cache middleware: dirty regeneration canceled for %v: %v", r.URL.Path, err)
				return
			}
			defer unlock()
			tileUpdateLocks.beginRegeneration(state)
			defer tileUpdateLocks.endRegeneration(state)

			res := renderTileForCache(r.Context(), r, next, cacher, key, false)
			res.writeTo(w)
			return
		}

		// Preserve the existing behavior for ordinary query parameters. A
		// falsy ?dirty value does not count: it is inert, so the request is
		// served exactly like one without the parameter.
		if hasDirty {
			query.Del(QueryKeyDirty)
		}
		if len(query) > 0 || (r.URL.RawQuery != "" && !hasDirty) {
			next.ServeHTTP(w, r)
			return
		}

		// Cache hits are served without holding any lock: reads and response
		// writes never block each other (or wait behind an update).

		// use the URL path as the key
		cachedTile, hit, err := cacher.Get(r.Context(), key)
		if err != nil {
			log.Errorf("cache middleware: error reading from cache: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		if !hit && key.LayerName != "" {
			cachedTile, hit, err = cachedMapLayer(r.Context(), a, cacher, key)
			if err != nil {
				log.Warnf("cache middleware: cannot reuse map tile for %s: %v", key.String(), err)
			}
		}

		if hit {
			// mimetype for mapbox vector tiles
			w.Header().Add("Content-Type", mvt.MimeType)

			// communicate the cache is being used
			w.Header().Add("Tegola-Cache", "HIT")
			w.Header().Add("Content-Length", fmt.Sprintf("%d", len(cachedTile)))

			_, _ = w.Write(cachedTile)
			return
		}

		// cache miss: render the tile once per tile key and share the captured
		// result with all concurrent requests for the same tile. The render is
		// driven by a context detached from any single request, so a leader
		// that disconnects cannot abort a render live waiters depend on.
		res, shared := tileRenders.do(r.Context(), cacheCoordinationKey(cacher, key), func(renderCtx context.Context) *tileRenderResult {
			return renderTileForCache(renderCtx, r, next, cacher, key, true)
		})

		if r.Context().Err() != nil {
			// our request ended while the render was in flight: the client is
			// gone and there is nothing left to deliver
			return
		}
		if res.canceled {
			// The shared render was abandoned (bounded render timeout) while we
			// were still live. A canceled result must never be written as an
			// empty 200: answer with a proper error instead.
			log.Warnf("cache middleware: shared render for %v did not complete", r.URL.Path)
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		if shared {
			// we waited on another request's render: report SHARED instead of
			// the captured MISS so cache instrumentation can tell the two apart
			res = res.waiterView()
		}
		res.writeTo(w)
	})
}

// tileRenderResult is the outcome of a tile render, captured in full so it can
// be replayed to any number of waiting requests and stored in the cache.
type tileRenderResult struct {
	status int
	header http.Header
	body   []byte
	// canceled reports that the render was abandoned because its request
	// context ended before producing a complete response.
	canceled bool
}

// writeTo replays the captured response to w.
func (res *tileRenderResult) writeTo(w http.ResponseWriter) {
	for k, vals := range res.header {
		w.Header()[k] = vals
	}
	status := res.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if len(res.body) > 0 {
		_, _ = w.Write(res.body)
	}
}

// waiterView returns the result as seen by a request that waited on another
// request's render: identical status, headers and body except that the
// Tegola-Cache header reports SHARED instead of the captured MISS. The shared
// result itself is left untouched for the leader and other waiters.
func (res *tileRenderResult) waiterView() *tileRenderResult {
	out := *res
	out.header = res.header.Clone()
	out.header.Set("Tegola-Cache", "SHARED")
	return &out
}

// tileRenderCapture records a handler response instead of sending it. Headers
// are frozen when the response is committed, matching net/http semantics.
type tileRenderCapture struct {
	header http.Header
	frozen http.Header
	status int
	body   bytes.Buffer
}

func newTileRenderCapture() *tileRenderCapture {
	header := http.Header{}
	// communicate the cache is being used (miss); mirrors the old
	// tileCacheResponseWriter behavior of stamping MISS on the response
	header.Set("Tegola-Cache", "MISS")
	return &tileRenderCapture{header: header}
}

func (c *tileRenderCapture) Header() http.Header {
	return c.header
}

func (c *tileRenderCapture) WriteHeader(status int) {
	if c.status != 0 {
		return
	}
	c.status = status
	c.frozen = c.header.Clone()
}

func (c *tileRenderCapture) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
		c.frozen = c.header.Clone()
	}
	return c.body.Write(b)
}

func (c *tileRenderCapture) result(canceled bool) *tileRenderResult {
	status := c.status
	header := c.frozen
	if status == 0 {
		// nothing was written: net/http would answer 200 with the headers set
		status = http.StatusOK
		header = c.header
	}
	return &tileRenderResult{
		status:   status,
		header:   header,
		body:     c.body.Bytes(),
		canceled: canceled,
	}
}

// isMVTContentType reports whether the response Content-Type is a Mapbox
// Vector Tile payload. Only such responses are stored in the tile cache.
func isMVTContentType(header http.Header) bool {
	ct := header.Get("Content-Type")
	if ct == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mediaType == mvt.MimeType
}

// renderTileForCache renders a tile through the wrapped handler and stores the
// encoded result in the cache. The render is captured and replayed to the
// caller instead of being streamed, so concurrent requests can share it. The
// ctx drives both the render and the cache write: shared miss renders pass a
// context detached from (but value-derived from) the initiating request, while
// mutating paths pass the request's own context.
//
// When checkStale is set (ordinary miss renders) the cached write is claimed
// atomically with respect to metatile mutations: if a ?tile=update or ?dirty
// regeneration rewrote the metatile while the render was in flight, the stale
// write is dropped, so a slow render can never overwrite freshly regenerated
// tiles. Mutating paths (?dirty) pass checkStale=false: they hold the metatile
// lock and their write is authoritative.
func renderTileForCache(ctx context.Context, r *http.Request, next http.Handler, cacher cache.Interface, key *cache.Key, checkStale bool) *tileRenderResult {
	var snap metatileSnapshot
	if checkStale {
		// keep the metatile state alive across the render and remember its
		// generation for the staleness check below
		lockKey := cacheMetatileLockKey(cacher, key)
		state := tileUpdateLocks.retain(lockKey)
		defer tileUpdateLocks.release(lockKey, state)
		snap = tileUpdateLocks.snapshot(state)
	}

	// render with a query-parameter-free copy of the request (?dirty and
	// friends must not leak into the provider query); cancellation flows
	// through ctx
	renderReq := r.Clone(ctx)
	renderReq.URL.RawQuery = ""
	renderReq.URL.Fragment = ""
	renderReq.Form = nil
	renderReq.PostForm = nil

	capture := newTileRenderCapture()
	next.ServeHTTP(capture, renderReq)

	res := capture.result(ctx.Err() != nil)
	if res.canceled || res.status != http.StatusOK || len(res.body) == 0 {
		return res
	}

	// only MVT tiles belong in the tile cache
	if !isMVTContentType(res.header) {
		return res
	}

	if checkStale {
		// the freshness re-check and the cache write are one atomic step with
		// respect to metatile mutations: a ?tile=update or ?dirty
		// regeneration that rewrote this metatile while we were rendering
		// supersedes our result and the stale write is dropped
		wrote, err := tileUpdateLocks.writeStable(ctx, snap, func(ctx context.Context) error {
			return cacher.Set(ctx, key, res.body)
		})
		if err != nil {
			log.Warnf("cache response writer err: %v", err)
		}
		if !wrote {
			log.Debugf("cache middleware: skipping stale cache write for tile %v", key)
		}
		return res
	}

	if err := cacher.Set(ctx, key, res.body); err != nil {
		log.Warnf("cache response writer err: %v", err)
	}
	return res
}

// tileRenderTimeout bounds how long a shared render may outlive the requests
// that started it. It is a variable so tests can tighten the bound.
var tileRenderTimeout = 30 * time.Second

// tileRenderGroup deduplicates concurrent renders of the same tile key in
// flight. Waiters receive the leader's result.
//
// The render is detached from any single request's context (bounded by
// tileRenderTimeout): a leader that disconnects must not abort a render other
// requests are still waiting for. Every request that joins a render holds a
// refcount; when all of them disconnect before completion, the detached render
// is canceled. A waiter whose own context ends reports cancellation and never
// observes an aborted render's result.
type tileRenderGroup struct {
	mu    sync.Mutex
	calls map[string]*tileRenderCall
}

type tileRenderCall struct {
	done chan struct{}
	res  *tileRenderResult
	// refs counts the requests still waiting on the render. The render runs
	// detached from request contexts; when the last waiter disconnects, the
	// detached render is canceled and the call is retired.
	refs   int
	cancel context.CancelFunc
}

// runTileRender runs fn, converting a panic into a proper 500 result so live
// waiters are never left without a response.
func runTileRender(ctx context.Context, fn func(context.Context) *tileRenderResult) (res *tileRenderResult) {
	defer func() {
		if p := recover(); p != nil {
			log.Errorf("cache middleware: tile render panicked: %v", p)
			res = &tileRenderResult{status: http.StatusInternalServerError}
		}
	}()
	return fn(ctx)
}

// do executes fn for key unless an identical call is already in flight, in
// which case it waits for and returns that call's result. The second return
// value reports whether the result was shared from another call.
func (g *tileRenderGroup) do(ctx context.Context, key string, fn func(context.Context) *tileRenderResult) (res *tileRenderResult, shared bool) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*tileRenderCall)
	}
	if call, ok := g.calls[key]; ok {
		call.refs++
		g.mu.Unlock()
		return call.wait(ctx, g, key), true
	}

	renderCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tileRenderTimeout)
	call := &tileRenderCall{done: make(chan struct{}), refs: 1, cancel: cancel}
	g.calls[key] = call
	g.mu.Unlock()

	go func() {
		defer cancel()
		call.res = runTileRender(renderCtx, fn)

		g.mu.Lock()
		if g.calls[key] == call {
			delete(g.calls, key)
		}
		g.mu.Unlock()
		close(call.done)
	}()

	return call.wait(ctx, g, key), false
}

// wait blocks until the render completes or ctx ends. A caller whose context
// ends first gives up its refcount and receives a canceled result; when the
// last ref is dropped the detached render is canceled and the call is retired
// so no new caller can join it.
func (c *tileRenderCall) wait(ctx context.Context, g *tileRenderGroup, key string) *tileRenderResult {
	select {
	case <-c.done:
		return c.res
	case <-ctx.Done():
		g.mu.Lock()
		c.refs--
		last := c.refs == 0
		if last && g.calls[key] == c {
			delete(g.calls, key)
		}
		g.mu.Unlock()
		if last {
			// no request is waiting for this render anymore: stop rendering
			c.cancel()
		}
		return &tileRenderResult{canceled: true}
	}
}
