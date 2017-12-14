package tegola

import (
	"fmt"
	"math"

	"github.com/terranodo/tegola/maths/webmercator"
)

const (
	DefaultEpislon    = 10.0
	DefaultExtent     = 4096
	DefaultTileBuffer = 32
	MaxZ              = 20
)

var UnknownConversionError = fmt.Errorf("do not know how to convert value to requested value")

//Tile slippy map tilenames
//	http://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
type Tile struct {
	Z         int
	X         int
	Y         int
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
	extent  [2][2]float64
	bufpext [2][2]float64
}

func NewTile(z, x, y int) (t *Tile) {
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
func NewTileLatLong(z int, lat, lon float64) (t *Tile) {
	t = &Tile{
		Z:         z,
		Lat:       lat,
		Long:      lon,
		Buffer:    DefaultTileBuffer,
		Extent:    DefaultExtent,
		Tolerance: DefaultEpislon,
	}
	t.X, t.Y = t.Deg2Num()
	t.Init()
	return t
}

func (t *Tile) Init() {
	max := 20037508.34

	//	resolution
	res := (max * 2) / math.Exp2(float64(t.Z))
	t.cached = true
	t.extent = [2][2]float64{
		{
			-max + (float64(t.X) * res), // MinX
			max - (float64(t.Y) * res),  // Miny
		},
		{
			-max + (float64(t.X) * res) + res, // MaxX
			max - (float64(t.Y) * res) - res,  // MaxY

		},
	}
	t.xspan = t.extent[1][0] - t.extent[0][0]
	t.yspan = t.extent[1][1] - t.extent[0][1]
	t.bufpext = [2][2]float64{{0 - t.Buffer, 0 - t.Buffer}, {t.Extent + t.Buffer, t.Extent + t.Buffer}}

}

func (t *Tile) Deg2Num() (x, y int) {
	x = int(math.Floor((t.Long + 180.0) / 360.0 * (math.Exp2(float64(t.Z)))))
	y = int(math.Floor((1.0 - math.Log(math.Tan(t.Lat*math.Pi/180.0)+1.0/math.Cos(t.Lat*math.Pi/180.0))/math.Pi) / 2.0 * (math.Exp2(float64(t.Z)))))

	return x, y
}

func (t *Tile) Num2Deg() (lat, lng float64) {
	n := math.Pi - 2.0*math.Pi*float64(t.Y)/math.Exp2(float64(t.Z))

	lat = 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lng = float64(t.X)/math.Exp2(float64(t.Z))*360.0 - 180.0

	return lat, lng
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

	nx := int64((spt[0] - t.extent[0][0]) * t.Extent / t.xspan)
	ny := int64((spt[1] - t.extent[0][1]) * t.Extent / t.yspan)
	return [2]float64{float64(nx), float64(ny)}, nil
}
func (t *Tile) FromPixel(srid int, pt [2]float64) (npt [2]float64, err error) {

	x := float64(int64(pt[0]))
	y := float64(int64(pt[1]))

	/*
		        n = (spt[0] - t.extent[0][0])
			z = t.Extent / t.xspan
			x = n * z
			x/z = n
			x/z = spt[0] - t.extent[0][0]
			x/z + t.extent[0][0] = spt[0]
			(x * t.xspan / t.Extent) + t.extent[0][0] = spt[0]
	*/
	wmx := (x * t.xspan / t.Extent) + t.extent[0][0]
	wmy := (y * t.yspan / t.Extent) + t.extent[0][1]
	return fromWebMercator(srid, [2]float64{wmx, wmy})

}

//BoundingBox returns the bound box coordinates for upper left (ulx, uly) and lower right (lrx, lry)
// in web mercator projection
// ported from: https://raw.githubusercontent.com/mapbox/postgis-vt-util/master/postgis-vt-util.sql
func (t *Tile) BoundingBox() BoundingBox {
	return BoundingBox{
		Minx:    t.extent[0][0],
		Miny:    t.extent[0][1],
		Maxx:    t.extent[1][0],
		Maxy:    t.extent[1][1],
		Epsilon: t.ZEpislon(),
		HasXYZ:  true,
		X:       t.X,
		Y:       t.Y,
		Z:       t.Z,
	}
}

func (t *Tile) BufferedBoundingBox() (BoundingBox, error) {

	pbounds, err := t.PixelBufferedBounds()
	if err != nil {
		return BoundingBox{}, err
	}

	min, err := t.FromPixel(WebMercator, pbounds[0])
	if err != nil {
		return BoundingBox{}, err
	}
	max, err := t.FromPixel(WebMercator, pbounds[1])
	if err != nil {
		return BoundingBox{}, err
	}

	return BoundingBox{
		Minx:    min[0],
		Miny:    min[1],
		Maxx:    max[0],
		Maxy:    max[1],
		Epsilon: t.ZEpislon(),
		HasXYZ:  true,
		X:       t.X,
		Y:       t.Y,
		Z:       t.Z,
	}, nil
}

func (t *Tile) PixelBounds() (bounds [2][2]float64, err error) {
	/*
			// This is how we can calculate it. But, it will always be a constant.
			// So, we just return that constant.
		bounds[0], err = t.ToPixel(WebMercator, t.extent[0])
		if err != nil {
			return bounds, err
		}
		bounds[1], err = t.ToPixel(WebMercator, t.extent[1])
		if err != nil {
			return bounds, err
		}
		log.Println("Z", t.Z, "X", t.X, "Y", t.Y)
		log.Println("Tile extent:", t.extent)
		log.Println("Tile Pixel :", bounds)
	*/
	return [2][2]float64{{0.0, 0.0}, {t.Extent, t.Extent}}, nil
}

func (t *Tile) PixelBufferedBounds() (bounds [2][2]float64, err error) {
	/*
		// This is how we can calculate it. But, it will always be a constant.
		// So, we just return that constant.
		bounds, err = t.PixelBounds()
		if err != nil {
			return bounds, err
		}
		bounds[0][0] -= t.Buffer
		bounds[0][1] -= t.Buffer
		bounds[1][0] += t.Buffer
		bounds[1][1] += t.Buffer
		return bounds, nil
	*/
	return t.bufpext, nil
}

//ZRes takes a web mercator zoom level and returns the pixel resolution for that
//	scale, assuming 256x256 pixel tiles. Non-integer zoom levels are accepted.
//	ported from: https://raw.githubusercontent.com/mapbox/postgis-vt-util/master/postgis-vt-util.sql
// TODO: gdey — I'm pretty sure we should be using the extent instead of 256 here. But I don't know what the magic number 40075016.6855785 is used for.
// 40075016.6855785 is close to 2*webmercator.MaxXExtent or 2*webmercator.MaxYExtent
func (t *Tile) ZRes() float64 {
	return 40075016.6855785 / (256 * math.Exp2(float64(t.Z)))
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
