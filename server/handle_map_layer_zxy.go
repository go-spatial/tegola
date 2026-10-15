package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/proj"
	"github.com/dimfeld/httptreemux"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/maths"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/provider"
)

var (
	webmercatorGrid = slippy.NewGrid(3857, 0)
	tileUpdateLocks = newTileUpdateCoordinator()
)

type HandleMapLayerZXY struct {
	// required
	mapName string
	// optional
	layerName string
	// zoom
	z uint
	// row
	x uint
	// column
	y uint
	// the requests extension (i.e. pbf or json)
	// defaults to "pbf"
	extension string
	// debug
	debug bool
	// the Atlas to use, nil (default) is the default atlas
	Atlas *atlas.Atlas
	// Optional cache override; Atlas cache remains the default.
	Cache cache.Interface
}

const (
	tileOperationStatus     = "status"
	tileOperationUpdate     = "update"
	tileOperationGetUpdated = "getupdated"
	metatileSize            = uint(8)
)

type tileUpdateCoordinator struct {
	mu        sync.Mutex
	metatiles map[string]*metatileState
}

// metatileState tracks the update and render state for one metatile (a
// metatileSize x metatileSize group of tiles sharing a cache key prefix).
type metatileState struct {
	// key is the coordinator map key this state was created for.
	key string
	// sem serializes metatile cache mutations ( ?tile=update / ?tile=getupdated
	// and ?dirty regeneration ). Ordinary cache reads and miss renders never
	// take it, so cache hits are always served without waiting on anyone.
	sem chan struct{}
	// refs counts outstanding holders of this state: mutation holders and
	// waiters plus in-flight miss renders that keep the state alive for
	// generation checks. The entry is dropped when refs falls to zero.
	refs int
	// generation is bumped at the start and at the end of every cache
	// mutation. Miss renders snapshot it before rendering and refuse to write
	// their result when it changed in the meantime, so a render started before
	// ?tile=update can never overwrite the freshly regenerated tiles with
	// stale bytes.
	generation uint64
	// updates counts in-flight metatile update operations ( ?tile=update and
	// ?tile=getupdated ) and backs the ?tile=status "updating" flag. Ordinary
	// requests are deliberately not counted: they do not update anything.
	updates int
	// mutations counts all in-flight cache mutations ( updates plus ?dirty ),
	// including those not reported as "updating".
	mutations int
}

// metatileSnapshot captures the mutation state of a metatile at a point in
// time so a miss render can later decide whether its result is still fresh.
type metatileSnapshot struct {
	state *metatileState
	gen   uint64
}

func newTileUpdateCoordinator() *tileUpdateCoordinator {
	return &tileUpdateCoordinator{metatiles: make(map[string]*metatileState)}
}

// retain returns the state for key, creating it when needed, and registers a
// reference. Every retain must be paired with a release.
func (c *tileUpdateCoordinator) retain(key string) *metatileState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.retainLocked(key)
}

func (c *tileUpdateCoordinator) retainLocked(key string) *metatileState {
	state := c.metatiles[key]
	if state == nil {
		state = &metatileState{key: key, sem: make(chan struct{}, 1)}
		c.metatiles[key] = state
	}
	state.refs++
	return state
}

func (c *tileUpdateCoordinator) release(key string, state *metatileState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state.refs--
	if state.refs == 0 && c.metatiles[key] == state {
		delete(c.metatiles, key)
	}
}

// acquire locks the metatile for a cache mutation. The returned release
// function is safe to call more than once. Waiting for the lock honors ctx
// cancellation, so a canceled request never stays stuck in the wait queue.
func (c *tileUpdateCoordinator) acquire(ctx context.Context, key string) (state *metatileState, release func(), err error) {
	state = c.retain(key)

	select {
	case state.sem <- struct{}{}:
	case <-ctx.Done():
		c.release(key, state)
		return nil, nil, ctx.Err()
	}

	var once sync.Once
	release = func() {
		once.Do(func() {
			<-state.sem
			c.release(key, state)
		})
	}
	return state, release, nil
}

// beginUpdate marks the start of a metatile update operation ( ?tile=update or
// ?tile=getupdated ). It must run while holding the metatile lock and be
// paired with endUpdate.
func (c *tileUpdateCoordinator) beginUpdate(state *metatileState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state.mutations++
	state.generation++
	state.updates++
}

// endUpdate marks the end of a metatile update operation.
func (c *tileUpdateCoordinator) endUpdate(state *metatileState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state.mutations--
	state.generation++
	state.updates--
}

// beginRegeneration marks the start of a ?dirty regeneration (a cache mutation
// that is not reported as "updating"). It must run while holding the metatile
// lock and be paired with endRegeneration.
func (c *tileUpdateCoordinator) beginRegeneration(state *metatileState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state.mutations++
	state.generation++
}

// endRegeneration marks the end of a ?dirty regeneration.
func (c *tileUpdateCoordinator) endRegeneration(state *metatileState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state.mutations--
	state.generation++
}

// snapshot captures the metatile generation for a later stable check. The
// state must be retained by the caller while the snapshot is in use.
func (c *tileUpdateCoordinator) snapshot(state *metatileState) metatileSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return metatileSnapshot{state: state, gen: state.generation}
}

// stable reports whether the metatile cache was left untouched since the
// snapshot was taken: no mutation ran or is running and the generation did
// not move. Miss renders use it to avoid writing stale bytes over tiles
// regenerated while they were rendering.
func (c *tileUpdateCoordinator) stable(snap metatileSnapshot) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := snap.state
	if state == nil {
		return false
	}
	return state.mutations == 0 && state.generation == snap.gen && c.metatiles[state.key] == state
}

// writeStable performs the cache write for a finished miss render when the
// metatile is still untouched since snap. The final freshness check and the
// write itself run under the metatile mutation lock, closing the window
// between the generation check and the cache write: a metatile mutation
// either completes before the check (the superseded write is dropped) or
// starts after the write (its regenerated tiles land last). A render from a
// superseded generation can therefore never overwrite newer tiles.
//
// set runs at most once, while the write is claimed; its error is returned to
// the caller. When the metatile was mutated since snap, or ctx ends before
// the write is claimed, nothing is written and wrote is false.
func (c *tileUpdateCoordinator) writeStable(ctx context.Context, snap metatileSnapshot, set func(context.Context) error) (wrote bool, err error) {
	state := snap.state
	if state == nil || !c.stable(snap) {
		return false, nil
	}

	// serialize with mutation writes ( ?tile=update / ?tile=getupdated and
	// ?dirty regeneration hold this lock across their cache writes )
	_, unlock, err := c.acquire(ctx, state.key)
	if err != nil {
		// the writer's context ended while waiting for the mutation lock
		return false, nil
	}
	defer unlock()

	// re-check under the mutation lock: anything that moved the generation
	// since the snapshot supersedes this render
	if !c.stable(snap) {
		return false, nil
	}
	return true, set(ctx)
}

// isUpdating reports whether a metatile update operation ( ?tile=update or
// ?tile=getupdated ) is currently in flight for key. Ordinary requests and
// ?dirty regenerations are not reported as updating.
func (c *tileUpdateCoordinator) isUpdating(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.metatiles[key]
	return state != nil && state.updates > 0
}

// parseURI reads the request URI and extracts the various values for the request
func (req *HandleMapLayerZXY) parseURI(r *http.Request) error {
	var err error

	params := httptreemux.ContextParams(r.Context())

	// set map name
	req.mapName = params["map_name"]
	req.layerName = params["layer_name"]

	var placeholder uint64

	z := params["z"]
	placeholder, err = strconv.ParseUint(z, 10, 32)
	if err != nil || placeholder > tegola.MaxZ {
		log.Warnf("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
	}
	req.z = uint(placeholder)

	maxXYatZ := maths.Exp2(placeholder) - 1

	x := params["x"]
	placeholder, err = strconv.ParseUint(x, 10, 32)
	if err != nil || placeholder > maxXYatZ {
		log.Warnf("invalid X value (%v)", x)
		return fmt.Errorf("invalid X value (%v)", x)
	}
	req.x = uint(placeholder)

	// trim the "y" param in the url in case it has an extension
	y := params["y"]
	yParts := strings.Split(y, ".")
	placeholder, err = strconv.ParseUint(yParts[0], 10, 32)
	if err != nil || placeholder > maxXYatZ {
		log.Warnf("invalid Y value (%v)", yParts[0])
		return fmt.Errorf("invalid Y value (%v)", yParts[0])
	}

	req.y = uint(placeholder)

	// check if we have a file extension
	if len(yParts) > 1 && yParts[len(yParts)-1] != "" {
		req.extension = yParts[len(yParts)-1]
	} else {
		req.extension = "pbf"
	}

	// Only MVT ("pbf") tile output is implemented. Other extensions (e.g. "json")
	// are not supported and are served as pbf; warn so the fallback is not silent.
	// This compatibility fallback is documented in server/README.md.
	if req.extension != "pbf" {
		log.Warnf("unsupported tile extension %q; serving tile as pbf (mvt)", req.extension)
	}

	// check for debug request
	if r.URL.Query().Get(QueryKeyDebug) == "true" {
		req.debug = true
	}

	return nil
}

// URI scheme: /maps/:map_name/:layer_name/:z/:x/:y?param=value
// map_name - map name in the config file
// layer_name - name of the single map layer to render
// z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
//
//	z - zoom level
//	x - row
//	y - column
//
// param - configurable query parameters and their values
func (req HandleMapLayerZXY) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// parse our URI
	if err := req.parseURI(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// lookup our Map
	m, err := req.Atlas.Map(req.mapName)
	if err != nil {
		errMsg := fmt.Sprintf("map (%v) not configured. check your config file", req.mapName)
		log.Error(errMsg)
		http.Error(w, errMsg, http.StatusNotFound)
		return
	}

	// filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(slippy.Zoom(req.z))
	if len(m.Layers) == 0 {
		msg := fmt.Sprintf("map (%v) has no layers, at zoom %v", req.mapName, req.z)
		log.Debug(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	if req.layerName != "" {
		m = m.FilterLayersByName(req.layerName)
		if len(m.Layers) == 0 {
			msg := fmt.Sprintf("map (%v) has no layers, for LayerName %v at zoom %v", req.mapName, req.layerName, req.z)
			log.Debug(msg)
			http.Error(w, msg, http.StatusNotFound)
			return

		}
	}

	tile := slippy.Tile{Z: slippy.Zoom(req.z), X: req.x, Y: req.y}

	{
		// Check to see that the zxy is within the bounds of the map.
		inBounds, err := tileWithinMapBounds(&m, tile)
		if err != nil {
			msg := fmt.Sprintf("map (%v -- %v) does not contains tile at %v/%v/%v. %v", req.mapName, m.Bounds, req.z, req.x, req.y, err)
			log.Debug(msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}
		if !inBounds {
			msg := fmt.Sprintf("map (%v -- %v) does not contains tile at %v/%v/%v", req.mapName, m.Bounds, req.z, req.x, req.y)
			log.Debug(msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}
	}

	// check for the debug query string
	if req.debug {
		m = m.AddDebugLayers()
	}

	operation, hasOperation := r.URL.Query()[QueryKeyTile]
	if hasOperation {
		if err := validateTileOperationQuery(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.debug {
			http.Error(w, "tile operations cannot be combined with debug", http.StatusBadRequest)
			return
		}

		// tile operations drive cache maintenance and are disabled by default;
		// when enabled they require the configured token and are rate limited.
		release, err := gateTileOperation(r, operation[0] != tileOperationStatus)
		if err != nil {
			log.Debugf("tile operation %v denied for map %v: %v", operation[0], req.mapName, err)
			writeTileOperationDenied(w, err)
			return
		}
		defer release()

		if err := req.serveTileOperation(w, r, m, tile, operation[0]); err != nil {
			switch {
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				return
			case errors.Is(err, errMetatileRegenUnavailable):
				// the regeneration scheduler is saturated or shutting down:
				// shed the request instead of queueing unbounded work
				log.Debugf("tile operation %v denied for map %v: %v", operation[0], req.mapName, err)
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
			default:
				log.Error(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		return
	}

	// check for query parameters and populate param map with their values
	params, err := extractParameters(m, r)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	encodeCtx := context.WithValue(r.Context(), observability.ObserveCtxKey(observability.ObserveVarMapName), m.Name)
	pbyte, err := m.Encode(encodeCtx, tile, params)

	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			log.Debugf("tile encode canceled for map %v z:%v x:%v y:%v", req.mapName, req.z, req.x, req.y)
			return
		case strings.Contains(err.Error(), "operation was canceled"):
			log.Debugf("tile encode canceled for map %v z:%v x:%v y:%v", req.mapName, req.z, req.x, req.y)
			return
		default:
			errMsg := fmt.Sprintf("error marshalling tile: %v", err)
			log.Error(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}

	// mimetype for mapbox vector tiles
	// https://www.iana.org/assignments/media-types/application/vnd.mapbox-vector-tile
	w.Header().Add("Content-Type", mvt.MimeType)
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(pbyte)))
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(pbyte)
	if err != nil {
		log.Errorf("error writing tile z:%v, x:%v, y:%v - %v", req.z, req.x, req.y, err)
	}

	// check for tile size warnings
	if len(pbyte) > MaxTileSize {
		log.Logger().Info("tile is rather large",
			slog.String("map", req.mapName),
			slog.String("layer", req.layerName),
			slog.Uint64("z", uint64(req.z)),
			slog.Uint64("x", uint64(req.x)),
			slog.Uint64("y", uint64(req.y)),
			slog.Int("size_kb", len(pbyte)/1024),
		)
	}
}

func validateTileOperationQuery(r *http.Request) error {
	query := r.URL.Query()
	operations, ok := query[QueryKeyTile]
	if len(query) != 1 || !ok || len(operations) != 1 {
		return fmt.Errorf("%s cannot be combined with other query parameters", QueryKeyTile)
	}

	switch operations[0] {
	case tileOperationStatus, tileOperationUpdate, tileOperationGetUpdated:
		return nil
	default:
		return fmt.Errorf("invalid %s operation %q", QueryKeyTile, operations[0])
	}
}

type tileStatusResponse struct {
	Map      string  `json:"map"`
	Layer    string  `json:"layer,omitempty"`
	Z        uint    `json:"z"`
	X        uint    `json:"x"`
	Y        uint    `json:"y"`
	Cached   bool    `json:"cached"`
	Updating bool    `json:"updating"`
	Metatile [4]uint `json:"metatile"`
}

func (req HandleMapLayerZXY) serveTileOperation(w http.ResponseWriter, r *http.Request, m atlas.Map, tile slippy.Tile, operation string) error {
	cacher := req.getCache(r.Context())
	// Maintenance uses the same request generation as ordinary reads, even
	// when an editor header asks ordinary tile reads to bypass that cache.
	w.Header().Set("Cache-Control", "no-store")
	key := req.tileCacheKey(tile)
	metatileKey := cacheMetatileLockKey(cacher, &key)

	if operation == tileOperationStatus {
		cached := false
		if cacher != nil {
			var err error
			_, cached, err = cacher.Get(r.Context(), &key)
			if err != nil {
				return fmt.Errorf("read tile status from cache: %w", err)
			}
			if !cached && key.LayerName != "" {
				_, cached, err = cachedMapLayer(r.Context(), req.Atlas, cacher, &key)
				if err != nil {
					return fmt.Errorf("read map-layer status from cache: %w", err)
				}
			}
		}

		baseX := (tile.X / metatileSize) * metatileSize
		baseY := (tile.Y / metatileSize) * metatileSize
		maxXY := uint(maths.Exp2(uint64(tile.Z)) - 1)
		endX := minUint(baseX+metatileSize-1, maxXY)
		endY := minUint(baseY+metatileSize-1, maxXY)
		status := tileStatusResponse{
			Map:      req.mapName,
			Layer:    req.layerName,
			Z:        uint(tile.Z),
			X:        tile.X,
			Y:        tile.Y,
			Cached:   cached,
			Updating: metatileRegens.isUpdating(metatileKey) || tileUpdateLocks.isUpdating(metatileKey),
			Metatile: [4]uint{baseX, baseY, endX - baseX + 1, endY - baseY + 1},
		}

		var jsonBuffer bytes.Buffer
		if err := json.NewEncoder(&jsonBuffer).Encode(status); err != nil {
			return fmt.Errorf("encode tile status: %w", err)
		}

		var gzipBuffer bytes.Buffer
		gzipWriter := gzip.NewWriter(&gzipBuffer)
		if _, err := gzipWriter.Write(jsonBuffer.Bytes()); err != nil {
			return fmt.Errorf("compress tile status: %w", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("finish compressed tile status: %w", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(gzipBuffer.Len()))
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(gzipBuffer.Bytes())
		return err
	}

	if cacher == nil {
		return fmt.Errorf("tile operation %q requires a configured cache", operation)
	}

	params, err := extractParameters(m, r)
	if err != nil {
		return fmt.Errorf("parse tile parameters: %w", err)
	}

	// hand the full metatile regeneration to the bounded background
	// scheduler: the request must not pay for up to metatileSize² renders
	// and cache writes while holding the metatile mutation lock. Concurrent
	// requests for the same metatile join the regeneration that is already
	// scheduled instead of starting another one.
	_, err = metatileRegens.enqueue(metatileKey, func(ctx context.Context) error {
		ctx = context.WithValue(ctx, observability.ObserveCtxKey(observability.ObserveVarMapName), m.Name)
		return regenerateMetatile(ctx, req, m, params, tile, cacher)
	})
	if err != nil {
		return err
	}

	if operation == tileOperationUpdate {
		w.Header().Del("Content-Encoding")
		w.Header().Del("Content-Length")
		w.WriteHeader(http.StatusAccepted)
		// body-capable responses must carry a complete gzip stream through
		// the gzip middleware (see GZipHandler and the ?tile=status handler),
		// so answer with an empty stream: gzip clients decode an empty body
		// and other clients receive an empty, correctly sized body.
		gzipWriter := gzip.NewWriter(w)
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("finish accepted response: %w", err)
		}
		return nil
	}

	// getupdated answers with the requested tile from a minimal single-tile
	// render — the same render an ordinary cache miss performs, shared with
	// any concurrent request for this tile — while the full metatile
	// regeneration proceeds in the background. Rendering through
	// renderTileForCache keeps the cache write metatile-lock safe.
	res, shared := tileRenders.do(r.Context(), cacheCoordinationKey(cacher, &key), func(renderCtx context.Context) *tileRenderResult {
		return renderTileForCache(renderCtx, r, http.HandlerFunc(req.ServeHTTP), cacher, &key, true)
	})
	if err := r.Context().Err(); err != nil {
		// our request ended while the render was in flight: the client is
		// gone and there is nothing left to deliver
		return err
	}
	if res.canceled {
		// the render was abandoned (bounded render timeout) while we were
		// still live: never write it as an empty 200
		log.Warnf("tile operation %v: shared render for %v did not complete", operation, r.URL.Path)
		w.WriteHeader(http.StatusGatewayTimeout)
		return nil
	}
	if shared {
		// we waited on another request's render: report SHARED instead of
		// the captured MISS so cache instrumentation can tell the two apart
		res = res.waiterView()
	}
	res.writeTo(w)
	return nil
}

// regenerateMetatile renders every in-bounds tile of the metatile containing
// tile and writes each result to the cache. It runs on the background
// scheduler (see metatileRegenScheduler) while ?tile=update and
// ?tile=getupdated requests return immediately. The render loop keeps the
// historical synchronous semantics: a failure on the requested tile aborts
// the pass, other out-of-bounds tiles are skipped, and any encode or cache
// error aborts the pass so a later request retries it.
func regenerateMetatile(ctx context.Context, req HandleMapLayerZXY, m atlas.Map, params provider.Params, tile slippy.Tile, cacher cache.Interface) error {
	key := req.tileCacheKey(tile)
	state, unlock, err := tileUpdateLocks.acquire(ctx, cacheMetatileLockKey(cacher, &key))
	if err != nil {
		return err
	}
	defer unlock()
	tileUpdateLocks.beginUpdate(state)
	defer tileUpdateLocks.endUpdate(state)

	maxXY := uint(maths.Exp2(uint64(tile.Z)) - 1)
	baseX := (tile.X / metatileSize) * metatileSize
	baseY := (tile.Y / metatileSize) * metatileSize
	endX := minUint(baseX+metatileSize-1, maxXY)
	endY := minUint(baseY+metatileSize-1, maxXY)

	for y := baseY; y <= endY; y++ {
		for x := baseX; x <= endX; x++ {
			if err := ctx.Err(); err != nil {
				return err
			}

			current := slippy.Tile{Z: tile.Z, X: x, Y: y}

			// only render and cache metatile tiles that fall within the map's
			// bounds; the requested tile is checked before we get here and
			// outside tiles would otherwise be encoded and cached needlessly
			inBounds, err := tileWithinMapBounds(&m, current)
			if err != nil {
				if current == tile {
					return err
				}
				log.Debugf("map (%v) does not contain tile %d/%d/%d: %v", req.mapName, current.Z, current.X, current.Y, err)
				continue
			}
			if !inBounds {
				if current == tile {
					return fmt.Errorf("map (%v -- %v) does not contain tile at %d/%d/%d", req.mapName, m.Bounds, current.Z, current.X, current.Y)
				}
				continue
			}

			encoded, err := m.Encode(ctx, current, params)
			if err != nil {
				return fmt.Errorf("encode tile %d/%d/%d: %w", current.Z, current.X, current.Y, err)
			}

			currentKey := req.tileCacheKey(current)
			if err := cacher.Set(ctx, &currentKey, encoded); err != nil {
				return fmt.Errorf("cache tile %d/%d/%d: %w", current.Z, current.X, current.Y, err)
			}
		}
	}

	return nil
}

func (req HandleMapLayerZXY) tileCacheKey(tile slippy.Tile) cache.Key {
	return cache.Key{
		MapName:   req.mapName,
		LayerName: req.layerName,
		Z:         uint(tile.Z),
		X:         tile.X,
		Y:         tile.Y,
	}
}

func (req HandleMapLayerZXY) metatileLockKey(tile slippy.Tile) string {
	return metatileLockKeyForCacheKey(&cache.Key{
		MapName:   req.mapName,
		LayerName: req.layerName,
		Z:         uint(tile.Z),
		X:         tile.X,
		Y:         tile.Y,
	})
}

func metatileLockKeyForCacheKey(key *cache.Key) string {
	baseX := (key.X / metatileSize) * metatileSize
	baseY := (key.Y / metatileSize) * metatileSize
	return fmt.Sprintf("%s/%s/%d/%d/%d", key.MapName, key.LayerName, key.Z, baseX, baseY)
}

// tileWithinMapBounds reports whether the given tile intersects the map's
// configured bounds. Maps without bounds contain every tile. Errors indicate
// the tile extent could not be computed or projected.
func tileWithinMapBounds(m *atlas.Map, tile slippy.Tile) (bool, error) {
	ext3857, err := slippy.Extent(webmercatorGrid, tile)
	if err != nil {
		return false, fmt.Errorf("unable to generate extent for tile %d/%d/%d: %w", tile.Z, tile.X, tile.Y, err)
	}

	points4326, err := proj.Inverse(proj.WebMercator, ext3857[:])
	if err != nil {
		return false, fmt.Errorf("unable to convert 3857 to 4326 for tile %d/%d/%d: %w", tile.Z, tile.X, tile.Y, err)
	}

	ext4326 := &geom.Extent{}
	copy(ext4326[:], points4326)
	return extentsIntersect(m.Bounds, ext4326), nil
}

// extentsIntersect reports whether a and b overlap. It matches the boolean
// result of geom.Extent.Intersect (edge-touching counts as no overlap) but
// without allocating a result extent. A nil extent is treated as the universe,
// matching the geom accessors.
func extentsIntersect(a, b *geom.Extent) bool {
	return !(a.MinX() >= b.MaxX() || b.MinX() >= a.MaxX() ||
		a.MinY() >= b.MaxY() || b.MinY() >= a.MaxY())
}

func minUint(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

func extractParameters(m atlas.Map, r *http.Request) (provider.Params, error) {
	var params provider.Params
	if len(m.Params) > 0 {
		params = make(provider.Params)
		err := r.ParseForm()
		if err != nil {
			return nil, err
		}

		for _, param := range m.Params {
			if r.Form.Has(param.Name) {
				val, err := param.ToValue(r.Form.Get(param.Name))
				if err != nil {
					return nil, err
				}
				params[param.Token] = val
			} else {
				p, err := param.ToDefaultValue()
				if err != nil {
					return nil, err
				}
				params[param.Token] = p
			}
		}
	}
	return params, nil
}

func (req HandleMapLayerZXY) getCache(ctx context.Context) cache.Interface {
	c := req.Cache
	if c == nil {
		c = req.Atlas.GetCache()
	}
	return requestTileCache(ctx, c)
}
