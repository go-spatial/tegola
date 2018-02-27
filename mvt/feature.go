package mvt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/geom/encoding/wkt"
	"github.com/go-spatial/tegola/internal/convert"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/points"
	"github.com/go-spatial/tegola/maths/validate"
	"github.com/go-spatial/tegola/mvt/vector_tile"
)

// errors
var (
	ErrNilFeature = fmt.Errorf("Feature is nil")
	// ErrUnknownGeometryType is the error retuned when the geometry is unknown.
	ErrUnknownGeometryType = fmt.Errorf("Unknown geometry type")
	ErrNilGeometryType     = fmt.Errorf("Nil geometry passed")
)

// TODO: Need to put in validation for the Geometry, at current the system
// does not check to make sure that the geometry is following the rules as
// laid out by the spec. It just assumes the user is good.

// Feature describes a feature of a Layer. A layer will contain multiple features
// each of which has a geometry describing the interesting thing, and the metadata
// associated with it.
type Feature struct {
	ID   *uint64
	Tags map[string]interface{}
	// Does not support the collection geometry, for this you have to create a feature for each
	// geometry in the collection.
	Geometry tegola.Geometry
	// Unsimplifed weather the Geometry is simple already and thus does not need to be simplified.
	Unsimplifed *bool
}

func wktEncode(g tegola.Geometry) string {
	gg, err := convert.ToGeom(g)
	if err != nil {
		return fmt.Sprintf("error converting tegola geom to geom geom, %v", err)
	}

	s, err := wkt.Encode(gg)
	if err != nil {
		return fmt.Sprintf("encoding error for geom geom, %v", err)
	}
	return s

}

func (f Feature) String() string {
	g := wktEncode(f.Geometry)
	if f.ID != nil {
		return fmt.Sprintf("{Feature: %v, GEO: %v, Tags: %+v}", *f.ID, g, f.Tags)
	}
	return fmt.Sprintf("{Feature: GEO: %v, Tags: %+v}", g, f.Tags)
}

//NewFeatures returns one or more features for the given Geometry
// TODO: Should we consider supporting validation of polygons and multiple polygons here?
func NewFeatures(geo tegola.Geometry, tags map[string]interface{}) (f []Feature) {
	if geo == nil {
		return f // return empty feature set for a nil geometry
	}

	if g, ok := geo.(tegola.Collection); ok {
		geos := g.Geometries()
		for i := range geos {
			f = append(f, NewFeatures(geos[i], tags)...)
		}
		return f
	}
	f = append(f, Feature{
		Tags:     tags,
		Geometry: geo,
	})
	return f
}

// VTileFeature will return a vectorTile.Feature that would represent the Feature
func (f *Feature) VTileFeature(ctx context.Context, keys []string, vals []interface{}, tile *tegola.Tile, simplify bool) (tf *vectorTile.Tile_Feature, err error) {
	tf = new(vectorTile.Tile_Feature)
	tf.Id = f.ID

	if tf.Tags, err = keyvalTagsMap(keys, vals, f); err != nil {
		return tf, err
	}

	geo, gtype, err := encodeGeometry(ctx, f.Geometry, tile, simplify)
	if err != nil {
		return tf, err
	}

	if len(geo) == 0 {
		return nil, nil
	}

	tf.Geometry = geo
	tf.Type = &gtype

	return tf, nil
}

// These values came from: https://github.com/mapbox/vector-tile-spec/tree/master/2.1
const (
	cmdMoveTo    uint32 = 1
	cmdLineTo    uint32 = 2
	cmdClosePath uint32 = 7

	maxCmdCount uint32 = 0x1FFFFFFF
)

type Command uint32

func NewCommand(cmd uint32, count int) Command {
	return Command((cmd & 0x7) | (uint32(count) << 3))
}

func (c Command) ID() uint32 {
	return uint32(c) & 0x7
}
func (c Command) Count() int {
	return int(uint32(c) >> 3)
}

func (c Command) String() string {
	switch c.ID() {
	case cmdMoveTo:
		return fmt.Sprintf("move Command with count %v", c.Count())
	case cmdLineTo:
		return fmt.Sprintf("line To command with count %v", c.Count())
	case cmdClosePath:
		return fmt.Sprintf("close path command with count %v", c.Count())
	default:
		return fmt.Sprintf("unknown command (%v) with count %v", c.ID(), c.Count())
	}
}

// encodeZigZag does the ZigZag encoding for small ints.
func encodeZigZag(i int64) uint32 {
	return uint32((i << 1) ^ (i >> 31))
}

// cursor reprsents the current position, this is needed to encode the geometry.
// 0,0 is the origin, it which is the top-left most part of the tile.
type cursor struct {
	// The coordinates — these should be int64, when they were float64 they
	// introduced a slight drift in the coordinates.
	x int64
	y int64

	// The tile of the screen.
	tile *tegola.Tile

	// Disabling scaling Use this when using clipping and scaling
	DisableScaling bool
}

func NewCursor(tile *tegola.Tile) *cursor {
	return &cursor{
		tile: tile,
	}
}

// GetDeltaPointAndUpdate assumes the Point is in WebMercator.
func (c *cursor) GetDeltaPointAndUpdate(p tegola.Point) (dx, dy int64) {
	var ix, iy int64
	var tx, ty = p.X(), p.Y()
	// TODO: gdey — We should get rid of this, as we generally disable scaling; now.
	if !c.DisableScaling {
		tpt, err := c.tile.ToPixel(tegola.WebMercator, [2]float64{tx, ty})
		if err != nil {
			// Conversion error most likly, need to panic.
			panic(err)
		}
		tx, ty = tpt[0], tpt[1]
	}
	ix, iy = int64(tx), int64(ty)
	// compute our point delta
	dx = ix - int64(c.x)
	dy = iy - int64(c.y)

	// update our cursor
	c.x = ix
	c.y = iy
	return dx, dy
}

func (c *cursor) scalept(g tegola.Point) basic.Point {
	pt, err := c.tile.ToPixel(tegola.WebMercator, [2]float64{g.X(), g.Y()})
	if err != nil {
		panic(err)
	}
	return basic.Point{pt[0], pt[1]}
}

func chk3Pts(pt1, pt2, pt3 basic.Point) int {
	// If the first and third points are equal we only care about
	// the first point.
	if tegola.IsPointEqual(pt1, pt3) {
		return 1
	}
	if tegola.IsPointEqual(pt1, pt2) || tegola.IsPointEqual(pt2, pt3) {
		return 2
	}
	return 3
}

func cleanLine(ols basic.Line) (newline basic.Line) {
	ls := ols
	loop := 0
Restart:
	count := 0
	//log.Println("Line:", ls.GoString())
	if len(ls) < 3 {
		for i := range ls {
			newline = append(newline, ls[i])
		}
		return newline
	}
	for i := 0; i < len(ls); i = i + 1 {
		//log.Println(len(ls), "I:", i)
		j, k := i+1, i+2
		switch {
		case i == len(ls)-2:
			k = 0
		case i == len(ls)-1:
			j, k = 0, 1
		}

		// Always add the first point.
		addFirstPt := true
		skip := 3 - chk3Pts(ls[i], ls[j], ls[k])
		//log.Println("Skip returned: ", skip, "I:", i)

		switch {
		case (k == 0 || k == 1) && skip == 2:
			addFirstPt = false
		case k == 1 && skip == 1:
			// remove the first point from newline
			newline = newline[1:]
		case skip == 0:
			count++
		}
		if addFirstPt {
			newline = append(newline, ls[i])
		}
		i += skip
		//log.Println(len(ls), "EI:", i)
	}
	//log.Println("Out of loop")

	if len(ls) != count {
		ls = newline
		newline = basic.Line{}
		loop++
		if loop > 100 {
			panic(fmt.Sprintf("infi (%v:%v)?\n%v\n%v", len(ls), count, ols.GoString(), ls.GoString()))
		}
		goto Restart
	}
	return newline
}

func simplifyLineString(g tegola.LineString, tolerance float64) basic.Line {
	line := basic.CloneLine(g)
	if len(line) <= 4 || maths.DistOfLine(g) < tolerance {
		return line
	}
	pts := line.AsPts()
	pts = maths.DouglasPeucker(pts, tolerance, true)
	if len(pts) == 0 {
		return nil
	}
	return basic.NewLineTruncatedFromPt(pts...)
}

func normalizePoints(pts []maths.Pt) (pnts []maths.Pt) {
	if pts[0] == pts[len(pts)-1] {
		pts = pts[1:]
	}
	if len(pts) <= 4 {
		return pts
	}
	lpt := 0
	pnts = append(pnts, pts[0])
	for i := 1; i < len(pts); i++ {
		ni := i + 1
		if ni >= len(pts) {
			ni = 0
		}
		m1, _, sdef1 := points.SlopeIntercept(pts[lpt], pts[i])
		m2, _, sdef2 := points.SlopeIntercept(pts[lpt], pts[ni])
		if m1 != m2 || sdef1 != sdef2 {
			pnts = append(pnts, pts[i])
		}
	}
	return pnts
}

func simplifyPolygon(g tegola.Polygon, tolerance float64, simplify bool) basic.Polygon {

	lines := g.Sublines()
	if len(lines) <= 0 {
		return nil
	}

	var poly basic.Polygon
	sqTolerance := tolerance * tolerance
	// First lets look the first line, then we will simplify the other lines.
	for i := range lines {
		area := maths.AreaOfPolygonLineString(lines[i])
		l := basic.CloneLine(lines[i])

		if area < sqTolerance {
			if i == 0 {
				return basic.ClonePolygon(g)
			}
			// don't simplify the internal line
			poly = append(poly, l)
			continue
		}

		pts := l.AsPts()
		if len(pts) <= 2 {
			if i == 0 {
				return nil
			}
			continue
		}
		pts = normalizePoints(pts)
		// If the last point is the same as the first, remove the first point.
		if len(pts) <= 4 {
			if i == 0 {
				return basic.ClonePolygon(g)
			}
			poly = append(poly, l)
			continue
		}

		pts = maths.DouglasPeucker(pts, sqTolerance, simplify)
		if len(pts) <= 2 {
			if i == 0 {
				return nil
			}
			//log.Println("\t Skipping polygon subline.")
			continue
		}

		poly = append(poly, basic.NewLineTruncatedFromPt(pts...))
	}

	if len(poly) == 0 {
		return nil
	}

	return poly
}

func SimplifyGeometry(g tegola.Geometry, tolerance float64, simplify bool) tegola.Geometry {
	if !simplify || g == nil {
		return g
	}
	switch gg := g.(type) {
	case tegola.Polygon:
		return simplifyPolygon(gg, tolerance, simplify)
	case tegola.MultiPolygon:
		var newMP basic.MultiPolygon
		for _, p := range gg.Polygons() {
			sp := simplifyPolygon(p, tolerance, simplify)
			if sp == nil {
				continue
			}
			newMP = append(newMP, sp)
		}
		if len(newMP) == 0 {
			return nil
		}
		return newMP
	case tegola.LineString:
		return simplifyLineString(gg, tolerance)
	case tegola.MultiLine:
		var newML basic.MultiLine
		for _, l := range gg.Lines() {
			sl := simplifyLineString(l, tolerance)
			if sl == nil {
				continue
			}
			newML = append(newML, sl)
		}
		if len(newML) == 0 {
			return nil
		}
		return newML
	}
	return g
}

func (c *cursor) scalelinestr(g tegola.LineString) (ls basic.Line) {

	pts := g.Subpoints()
	// If the linestring
	if len(pts) < 2 {
		// Not enought points to make a line.
		return nil
	}
	ls = make(basic.Line, 0, len(pts))
	ls = append(ls, c.scalept(pts[0]))
	lidx := len(ls) - 1
	for i := 1; i < len(pts); i++ {
		npt := c.scalept(pts[i])
		if tegola.IsPointEqual(ls[lidx], npt) {
			// drop any duplicate points.
			continue
		}
		ls = append(ls, npt)
		lidx = len(ls) - 1
	}

	if len(ls) < 2 {
		// Not enough points. the zoom must be too far out for this ring.
		return nil
	}
	return ls
}

func (c *cursor) scalePolygon(g tegola.Polygon) (p basic.Polygon) {

	lines := g.Sublines()
	p = make(basic.Polygon, 0, len(lines))

	if len(lines) == 0 {
		return p
	}
	for i := range lines {
		ln := c.scalelinestr(lines[i])
		if len(ln) < 2 {
			if debug {
				// skip lines that have been reduced to less then 2 points.
				log.Debug("skipping line 2", lines[i], len(ln))
			}
			continue
		}
		p = append(p, ln)
	}
	return p
}

func (c *cursor) ScaleGeo(geo tegola.Geometry) basic.Geometry {
	switch g := geo.(type) {
	case tegola.Point:
		return c.scalept(g)
	case tegola.Point3:
		return c.scalept(g)
	case tegola.MultiPoint:
		pts := g.Points()
		if len(pts) == 0 {
			return nil
		}
		var ptmap = make(map[basic.Point]struct{})
		var mp = make(basic.MultiPoint, 0, len(pts))
		mp = append(mp, c.scalept(pts[0]))
		ptmap[mp[0]] = struct{}{}
		for i := 1; i < len(pts); i++ {
			npt := c.scalept(pts[i])
			if _, ok := ptmap[npt]; ok {
				// Skip duplicate points.
				continue
			}
			ptmap[npt] = struct{}{}
			mp = append(mp, npt)
		}
		return mp
	case tegola.LineString:
		return c.scalelinestr(g)
	case tegola.MultiLine:
		var ml basic.MultiLine
		for _, l := range g.Lines() {
			nl := c.scalelinestr(l)
			if len(nl) > 0 {
				ml = append(ml, nl)
			}
		}
		return ml
	case tegola.Polygon:
		return c.scalePolygon(g)

	case tegola.MultiPolygon:
		var mp basic.MultiPolygon
		for _, p := range g.Polygons() {
			np := c.scalePolygon(p)
			if len(np) > 0 {
				mp = append(mp, np)
			}
		}
		return mp
	}
	return basic.G{}
}

type geoDebugStruct struct {
	Min maths.Pt       `json:"min"`
	Max maths.Pt       `json:"max"`
	Geo basic.Geometry `json:"geo"`
}

func createDebugFile(min, max maths.Pt, geo tegola.Geometry, err error) {
	fln := os.Getenv("GenTestCase")
	if fln == "" {
		return
	}
	filename := fmt.Sprintf("/tmp/testcase_%v_%p.json", fln, geo)
	bgeo, err := basic.CloneGeometry(geo)
	if err != nil {
		log.Errorf("failed to clone geo for test case. %v", err)
		return
	}
	f, err := os.Create(filename)
	if err != nil {
		log.Errorf("failed to create test file %v : %v.", filename, err)
		return
	}
	defer f.Close()
	geodebug := geoDebugStruct{
		Max: max,
		Min: min,
		Geo: bgeo,
	}
	enc := json.NewEncoder(f)
	enc.Encode(geodebug)
	log.Infof("created file: %v", filename)
}

func (c *cursor) encodeCmd(cmd uint32, points []tegola.Point) []uint32 {
	if len(points) == 0 {
		return []uint32{}
	}
	//	new slice to hold our encode bytes. 2 bytes for each point pluse a command byte.
	g := make([]uint32, 0, (2*len(points))+1)
	//	add the command integer
	g = append(g, cmd)

	//	range through our points
	for _, p := range points {
		dx, dy := c.GetDeltaPointAndUpdate(p)
		//	encode our delta point
		g = append(g, encodeZigZag(dx), encodeZigZag(dy))
	}
	return g
}

func (c *cursor) MoveTo(points ...tegola.Point) []uint32 {
	return c.encodeCmd(uint32(NewCommand(cmdMoveTo, len(points))), points)
}
func (c *cursor) LineTo(points ...tegola.Point) []uint32 {
	return c.encodeCmd(uint32(NewCommand(cmdLineTo, len(points))), points)
}
func (c *cursor) ClosePath() uint32 {
	return uint32(NewCommand(cmdClosePath, 1))
}

// encodeGeometry will take a tegola.Geometry type and encode it according to the
// mapbox vector_tile spec.
func encodeGeometry(ctx context.Context, geom tegola.Geometry, tile *tegola.Tile, simplify bool) (g []uint32, vtyp vectorTile.Tile_GeomType, err error) {

	if geom == nil {
		return nil, vectorTile.Tile_UNKNOWN, ErrNilGeometryType
	}

	//	new cursor
	c := NewCursor(tile)
	// We are scaling separately, no need to scale in cursor.
	c.DisableScaling = true

	// Project Geom

	// TODO: gdey: We need to separate out the transform, simplification, and clipping from the encoding process. #224

	geo := c.ScaleGeo(geom)
	sg := SimplifyGeometry(geo, tile.ZEpislon(), simplify)

	pbb, err := tile.PixelBufferedBounds()
	if err != nil {
		return nil, vectorTile.Tile_UNKNOWN, err
	}
	ext := points.Extent(pbb)

	geom, err = validate.CleanGeometry(ctx, sg, &ext)
	if err != nil {
		return nil, vectorTile.Tile_UNKNOWN, err
	}
	if geom == nil {
		return []uint32{}, -1, nil
	}
	switch t := geom.(type) {
	case tegola.Point:
		g = append(g, c.MoveTo(t)...)
		return g, vectorTile.Tile_POINT, nil

	case tegola.Point3:
		g = append(g, c.MoveTo(t)...)
		return g, vectorTile.Tile_POINT, nil

	case tegola.MultiPoint:
		g = append(g, c.MoveTo(t.Points()...)...)
		return g, vectorTile.Tile_POINT, nil

	case tegola.LineString:
		points := t.Subpoints()
		g = append(g, c.MoveTo(points[0])...)
		g = append(g, c.LineTo(points[1:]...)...)
		return g, vectorTile.Tile_LINESTRING, nil

	case tegola.MultiLine:
		lines := t.Lines()
		for _, l := range lines {
			points := l.Subpoints()
			g = append(g, c.MoveTo(points[0])...)
			g = append(g, c.LineTo(points[1:]...)...)
		}
		return g, vectorTile.Tile_LINESTRING, nil

	case tegola.Polygon:
		// TODO: Right now c.ScaleGeo() never returns a Polygon, so this is dead code.
		lines := t.Sublines()
		for _, l := range lines {
			points := l.Subpoints()
			g = append(g, c.MoveTo(points[0])...)
			g = append(g, c.LineTo(points[1:]...)...)
			g = append(g, c.ClosePath())
		}
		return g, vectorTile.Tile_POLYGON, nil

	case tegola.MultiPolygon:
		polygons := t.Polygons()
		for _, p := range polygons {
			lines := p.Sublines()
			for _, l := range lines {
				points := l.Subpoints()
				g = append(g, c.MoveTo(points[0])...)
				g = append(g, c.LineTo(points[1:]...)...)
				g = append(g, c.ClosePath())
			}
		}
		return g, vectorTile.Tile_POLYGON, nil

	default:
		return nil, vectorTile.Tile_UNKNOWN, ErrUnknownGeometryType
	}
}

// keyvalMapsFromFeatures returns a key map and value map, to help with the translation
// to mapbox tile format. In the Tile format, the Tile contains a mapping of all the unique
// keys and values, and then each feature contains a vector map to these two. This is an
// intermediate data structure to help with the construction of the three mappings.
func keyvalMapsFromFeatures(features []Feature) (keyMap []string, valMap []interface{}, err error) {
	var didFind bool
	for _, f := range features {
		for k, v := range f.Tags {
			didFind = false
			for _, mk := range keyMap {
				if k == mk {
					didFind = true
					break
				}
			}
			if !didFind {
				keyMap = append(keyMap, k)
			}
			didFind = false

			switch vt := v.(type) {
			default:
				if vt == nil {
					// ignore nil types
					continue
				}
				return keyMap, valMap, fmt.Errorf("unsupported type for value(%v) with key(%v) in tags for feature %v.", vt, k, f)

			case string:
				for _, mv := range valMap {
					tmv, ok := mv.(string)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case fmt.Stringer:
				for _, mv := range valMap {
					tmv, ok := mv.(fmt.Stringer)
					if !ok {
						continue
					}
					if tmv.String() == vt.String() {
						didFind = true
						break
					}
				}

			case int:
				for _, mv := range valMap {
					tmv, ok := mv.(int)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int8:
				for _, mv := range valMap {
					tmv, ok := mv.(int8)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int16:
				for _, mv := range valMap {
					tmv, ok := mv.(int16)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int32:
				for _, mv := range valMap {
					tmv, ok := mv.(int32)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int64:
				for _, mv := range valMap {
					tmv, ok := mv.(int64)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint:
				for _, mv := range valMap {
					tmv, ok := mv.(uint)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint8:
				for _, mv := range valMap {
					tmv, ok := mv.(uint8)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint16:
				for _, mv := range valMap {
					tmv, ok := mv.(uint16)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint32:
				for _, mv := range valMap {
					tmv, ok := mv.(uint32)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint64:
				for _, mv := range valMap {
					tmv, ok := mv.(uint64)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case float32:
				for _, mv := range valMap {
					tmv, ok := mv.(float32)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case float64:
				for _, mv := range valMap {
					tmv, ok := mv.(float64)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case bool:
				for _, mv := range valMap {
					tmv, ok := mv.(bool)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			} // value type switch

			if !didFind {
				valMap = append(valMap, v)
			}

		} // For f.Tags
	} // for features
	return keyMap, valMap, nil
}

// keyvalTagsMap will return the tags map as expected by the mapbox tile spec. It takes
// a keyMap and a valueMap that list the the order of the expected keys and values. It will
// return a vector map that refers to these two maps.
func keyvalTagsMap(keyMap []string, valueMap []interface{}, f *Feature) (tags []uint32, err error) {

	if f == nil {
		return nil, ErrNilFeature
	}

	var kidx, vidx int64

	for key, val := range f.Tags {

		kidx, vidx = -1, -1 // Set to known not found value.

		for i, k := range keyMap {
			if k != key {
				continue // move to the next key
			}
			kidx = int64(i)
			break // we found a match
		}

		if kidx == -1 {
			log.Errorf("did not find key (%v) in keymap.", key)
			return tags, fmt.Errorf("did not find key (%v) in keymap.", key)
		}

		// if val is nil we skip it for now
		// https://github.com/mapbox/vector-tile-spec/issues/62
		if val == nil {
			continue
		}

		for i, v := range valueMap {
			switch tv := val.(type) {
			default:
				return tags, fmt.Errorf("value (%[1]v) of type (%[1]T) for key (%[2]v) is not supported.", tv, key)
			case string:
				vmt, ok := v.(string) // Make sure the type of the Value map matches the type of the Tag's value
				if !ok || vmt != tv { // and that the values match
					continue // if they don't match move to the next value.
				}
			case fmt.Stringer:
				vmt, ok := v.(fmt.Stringer)
				if !ok || vmt.String() != tv.String() {
					continue
				}
			case int:
				vmt, ok := v.(int)
				if !ok || vmt != tv {
					continue
				}
			case int8:
				vmt, ok := v.(int8)
				if !ok || vmt != tv {
					continue
				}
			case int16:
				vmt, ok := v.(int16)
				if !ok || vmt != tv {
					continue
				}
			case int32:
				vmt, ok := v.(int32)
				if !ok || vmt != tv {
					continue
				}
			case int64:
				vmt, ok := v.(int64)
				if !ok || vmt != tv {
					continue
				}
			case uint:
				vmt, ok := v.(uint)
				if !ok || vmt != tv {
					continue
				}
			case uint8:
				vmt, ok := v.(uint8)
				if !ok || vmt != tv {
					continue
				}
			case uint16:
				vmt, ok := v.(uint16)
				if !ok || vmt != tv {
					continue
				}
			case uint32:
				vmt, ok := v.(uint32)
				if !ok || vmt != tv {
					continue
				}
			case uint64:
				vmt, ok := v.(uint64)
				if !ok || vmt != tv {
					continue
				}

			case float32:
				vmt, ok := v.(float32)
				if !ok || vmt != tv {
					continue
				}
			case float64:
				vmt, ok := v.(float64)
				if !ok || vmt != tv {
					continue
				}
			case bool:
				vmt, ok := v.(bool)
				if !ok || vmt != tv {
					continue
				}
			} // Values Switch Statement
			// if the values match let's record the index.
			vidx = int64(i)
			break // we found our value no need to continue on.
		} // range on value

		if vidx == -1 { // None of the values matched.
			return tags, fmt.Errorf("did not find a value: %v in valuemap.", val)
		}
		tags = append(tags, uint32(kidx), uint32(vidx))
	} // Move to the next tag key and value.

	return tags, nil
}
