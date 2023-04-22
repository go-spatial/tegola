package tegola

import (
	"fmt"
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/maths/webmercator"
)

const (
	DefaultEpislon    = 10.0
	DefaultExtent     = 4096
	DefaultTileBuffer = 64.0
	MaxZ              = 22
)

var UnknownConversionError = fmt.Errorf("do not know how to convert value to requested value")

// Tile slippy map tilenames
// http://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
type Tile struct {
	Z         uint
	X         uint
	Y         uint
	Lat       float64
	Long      float64
	Tolerance float64
	Extent    float64
	Buffer    float64

	// These values are cached
	cached bool
	// The width and height of the region.
	xspan float64
	yspan float64
	// This is the computed bounding box.
	extent  *geom.Extent
	bufpext *geom.Extent
}

// NewTile will return a non-nil tile object.
func NewTile(z, x, y uint) (t *Tile) {
	t = &Tile{
		Z:         z,
		X:         x,
		Y:         y,
		Buffer:    DefaultTileBuffer,
		Extent:    DefaultExtent,
		Tolerance: DefaultEpislon,
	}
	t.Lat, t.Long = t.Num2Deg()
	t.Init()
	return t
}

// NewTileLatLong will return a non-nil tile object.
func NewTileLatLong(z uint, lat, lon float64) (t *Tile) {
	t = &Tile{
		Z:         z,
		Lat:       lat,
		Long:      lon,
		Buffer:    DefaultTileBuffer,
		Extent:    DefaultExtent,
		Tolerance: DefaultEpislon,
	}
	x, y := t.Deg2Num()
	t.X, t.Y = uint(x), uint(y)
	t.Init()
	return t
}

func (t *Tile) Init() {
	max := 20037508.34

	// resolution
	res := (max * 2) / math.Exp2(float64(t.Z))
	t.cached = true
	t.extent = &geom.Extent{
		-max + (float64(t.X) * res),       // MinX
		max - (float64(t.Y) * res),        // Miny
		-max + (float64(t.X) * res) + res, // MaxX
		max - (float64(t.Y) * res) - res,  // MaxY

	}
	t.xspan = t.extent.MaxX() - t.extent.MinX()
	t.yspan = t.extent.MaxY() - t.extent.MinY()
	/*
		// This is how we can calculate it. But, it will always be a constant.
		// So, we just return that constant.
		// Where PixelBounds is :  [4]float64{0.0, 0.0, t.Extent, t.Extent}
		bounds, err = t.PixelBounds()
		if err != nil {
			return bounds, err
		}
		bounds[0][0] -= t.Buffer
		bounds[0][1] -= t.Buffer
		bounds[1][0] += t.Buffer
		bounds[1][1] += t.Buffer
	*/
	t.bufpext = &geom.Extent{
		0 - t.Buffer, 0 - t.Buffer,
		t.Extent + t.Buffer, t.Extent + t.Buffer,
	}
}

func (t *Tile) Deg2Num() (x, y int) {
	x = int(math.Floor((t.Long + 180.0) / 360.0 * (math.Exp2(float64(t.Z)))))
	y = int(math.Floor((1.0 - math.Log(math.Tan(t.Lat*math.Pi/180.0)+1.0/math.Cos(t.Lat*math.Pi/180.0))/math.Pi) / 2.0 * (math.Exp2(float64(t.Z)))))

	return x, y
}

func (t *Tile) Num2Deg() (lat, lng float64) {
	lat = Tile2Lat(uint64(t.Y), uint64(t.Z))
	lng = Tile2Lon(uint64(t.X), uint64(t.Z))
	return lat, lng
}

func Tile2Lon(x, z uint64) float64 { return float64(x)/math.Exp2(float64(z))*360.0 - 180.0 }

func Tile2Lat(y, z uint64) float64 {
	var n float64 = math.Pi
	if y != 0 {
		n = math.Pi - 2.0*math.Pi*float64(y)/math.Exp2(float64(z))
	}

	return 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
}

// Bounds returns the bounds of the Tile as defined by the North most Longitude, East most Latitude, South most Longitude, West most Latitude.
func (t *Tile) Bounds() [4]float64 {
	north := Tile2Lon(uint64(t.X), uint64(t.Z))
	east := Tile2Lat(uint64(t.Y), uint64(t.Z))
	south := Tile2Lon(uint64(t.X+1), uint64(t.Z))
	west := Tile2Lat(uint64(t.Y+1), uint64(t.Z))
	return [4]float64{north, east, south, west}
}

func toWebMercator(srid int, pt [2]float64) (npt [2]float64, err error) {
	switch srid {
	default:
		return npt, UnknownConversionError
	case WebMercator:
		return pt, nil
	case WGS84:
		tnpt, err := webmercator.PToXY(pt[0], pt[1])
		if err != nil {
			return npt, err
		}
		return [2]float64{tnpt[0], tnpt[1]}, nil
	}
}

func fromWebMercator(srid int, pt [2]float64) (npt [2]float64, err error) {
	switch srid {
	default:
		return npt, UnknownConversionError
	case WebMercator:
		return pt, nil
	case WGS84:
		tnpt, err := webmercator.PToLonLat(pt[0], pt[1])
		if err != nil {
			return npt, err
		}
		return [2]float64{tnpt[0], tnpt[1]}, nil
	}
}

func (t *Tile) ToPixel(srid int, pt [2]float64) (npt [2]float64, err error) {
	spt, err := toWebMercator(srid, pt)
	if err != nil {
		return npt, err
	}

	nx := int64((spt[0] - t.extent.MinX()) * t.Extent / t.xspan)
	ny := int64((spt[1] - t.extent.MinY()) * t.Extent / t.yspan)
	return [2]float64{float64(nx), float64(ny)}, nil
}

func (t *Tile) FromPixel(srid int, pt [2]float64) (npt [2]float64, err error) {

	x := float64(int64(pt[0]))
	y := float64(int64(pt[1]))

	wmx := (x * t.xspan / t.Extent) + t.extent.MinX()
	wmy := (y * t.yspan / t.Extent) + t.extent.MinY()
	return fromWebMercator(srid, [2]float64{wmx, wmy})

}

func (t *Tile) PixelBufferedBounds() (bounds [4]float64, err error) {
	return t.bufpext.Extent(), nil
}

// Returns web mercator zoom level
func (t *Tile) ZLevel() uint {
	return t.Z
}

// ZRes takes a web mercator zoom level and returns the pixel resolution for that
// scale, assuming t.Extent x t.Extent pixel tiles. Non-integer zoom levels are accepted.
// ported from: https://raw.githubusercontent.com/mapbox/postgis-vt-util/master/postgis-vt-util.sql
// 40075016.6855785 is the equator in meters for WGS84 at z=0
func (t *Tile) ZRes() float64 {
	return 40075016.6855785 / (t.Extent * math.Exp2(float64(t.Z)))
}

// This is from Leafty
func (t *Tile) ZEpislon() float64 {

	if t.Z == MaxZ {
		return 0
	}
	epi := t.Tolerance
	if epi <= 0 {
		return 0
	}
	ext := t.Extent

	denom := (math.Exp2(float64(t.Z)) * ext)

	e := epi / denom
	return e
}
