// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package proj

import (
	"fmt"
	"sync"

	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/support"

	// need to pull in the operations table entries
	_ "github.com/go-spatial/proj/operations"
)

// EPSGCode is the enum type for coordinate systems
type EPSGCode int

// Supported EPSG codes
const (
	EPSG3395                    EPSGCode = 3395
	WorldMercator                        = EPSG3395
	EPSG3857                    EPSGCode = 3857
	WebMercator                          = EPSG3857
	EPSG4087                    EPSGCode = 4087
	WorldEquidistantCylindrical          = EPSG4087
	EPSG4326                    EPSGCode = 4326
	WGS84                                = EPSG4326
)

// Convert performs a conversion from a 4326 coordinate system (lon/lat
// degrees, 2D) to the given projected system (x/y meters, 2D).
//
// The input is assumed to be an array of lon/lat points, e.g. [lon0, lat0,
// lon1, lat1, lon2, lat2, ...]. The length of the array must, therefore, be
// even.
//
// The returned output is a similar array of x/y points, e.g. [x0, y0, x1,
// y1, x2, y2, ...].
func Convert(dest EPSGCode, input []float64) ([]float64, error) {

	cacheLock.Lock()
	conv, err := newConversion(dest)
	cacheLock.Unlock()
	if err != nil {
		return nil, err
	}

	return conv.convert(input)
}

// Inverse converts from a projected X/Y of a coordinate system to
// 4326 (lat/lon, 2D).
//
// The input is assumed to be an array of x/y points, e.g. [x0, y0,
// x1, y1, x2, y2, ...]. The length of the array must, therefore, be
// even.
//
// The returned output is a similar array of lon/lat points, e.g. [lon0, lat0, lon1,
// lat1, lon2, lat2, ...].
func Inverse(src EPSGCode, input []float64) ([]float64, error) {
	cacheLock.Lock()
	conv, err := newConversion(src)
	cacheLock.Unlock()
	if err != nil {
		return nil, err
	}

	return conv.inverse(input)
}

// CustomProjection provides write-only access to the internal projection list
// so that projections may be added without having to modify the library code.
func CustomProjection(code EPSGCode, str string) {
	projStringLock.Lock()
	projStrings[code] = str
	projStringLock.Unlock()
}

//---------------------------------------------------------------------------

// conversion holds the objects needed to perform a conversion
type conversion struct {
	dest       EPSGCode
	projString *support.ProjString
	system     *core.System
	operation  core.IOperation
	converter  core.IConvertLPToXY
}

var (
	// cacheLock ensure only one person is updating our cache of converters at a time
	cacheLock   = sync.Mutex{}
	conversions = map[EPSGCode]*conversion{}

	projStringLock = sync.RWMutex{}
	projStrings    = map[EPSGCode]string{
		EPSG3395: "+proj=merc +lon_0=0 +k=1 +x_0=0 +y_0=0 +datum=WGS84",                            // TODO: support +units=m +no_defs
		EPSG3857: "+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k=1.0", // TODO: support +units=m +nadgrids=@null +wktext +no_defs
		EPSG4087: "+proj=eqc +lat_ts=0 +lat_0=0 +lon_0=0 +x_0=0 +y_0=0 +datum=WGS84",               // TODO: support +units=m +no_defs
	}
)

// AvailableConversions  returns a list of conversion that the system knows about
func AvailableConversions() (ret []EPSGCode) {
	projStringLock.RLock()
	defer projStringLock.RUnlock()
	if len(projStrings) == 0 {
		return nil
	}
	ret = make([]EPSGCode, 0, len(projStrings))
	for k := range projStrings {
		ret = append(ret, k)
	}
	return ret
}

// IsKnownConversionSRID returns if we know about the conversion
func IsKnownConversionSRID(srid EPSGCode) bool {
	projStringLock.RLock()
	defer projStringLock.RUnlock()
	_, ok := projStrings[srid]
	return ok
}

// newConversion creates a conversion object for the destination systems. If
// such a conversion already exists in the cache, use that.
func newConversion(dest EPSGCode) (*conversion, error) {

	projStringLock.RLock()
	str, ok := projStrings[dest]
	projStringLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("epsg code is not a supported projection")
	}

	conv, ok := conversions[dest]
	if ok {
		return conv, nil
	}

	// need to build it

	ps, err := support.NewProjString(str)
	if err != nil {
		return nil, err
	}

	sys, opx, err := core.NewSystem(ps)
	if err != nil {
		return nil, err
	}

	if !opx.GetDescription().IsConvertLPToXY() {
		return nil, fmt.Errorf("projection type is not supported")
	}

	conv = &conversion{
		dest:       dest,
		projString: ps,
		system:     sys,
		operation:  opx,
		converter:  opx.(core.IConvertLPToXY),
	}

	// cache it
	conversions[dest] = conv

	return conv, nil
}

// convert performs the projection on the given input points
func (conv *conversion) convert(input []float64) ([]float64, error) {

	if conv == nil || conv.converter == nil {
		return nil, fmt.Errorf("conversion not initialized")
	}

	if len(input)%2 != 0 {
		return nil, fmt.Errorf("input array of lon/lat values must be an even number")
	}

	output := make([]float64, len(input))

	lp := &core.CoordLP{}

	for i := 0; i < len(input); i += 2 {
		lp.Lam = support.DDToR(input[i])
		lp.Phi = support.DDToR(input[i+1])

		xy, err := conv.converter.Forward(lp)
		if err != nil {
			return nil, err
		}

		output[i] = xy.X
		output[i+1] = xy.Y
	}

	return output, nil
}

func (conv *conversion) inverse(input []float64) ([]float64, error) {
	if conv == nil || conv.converter == nil {
		return nil, fmt.Errorf("conversion not initialized")
	}

	if len(input)%2 != 0 {
		return nil, fmt.Errorf("input array of x/y values must be an even number")
	}

	output := make([]float64, len(input))

	xy := &core.CoordXY{}

	for i := 0; i < len(input); i += 2 {
		xy.X = input[i]
		xy.Y = input[i+1]

		lp, err := conv.converter.Inverse(xy)

		if err != nil {
			return nil, err
		}

		l, p := lp.Lam, lp.Phi

		output[i] = support.RToDD(l)
		output[i+1] = support.RToDD(p)
	}

	return output, nil
}
