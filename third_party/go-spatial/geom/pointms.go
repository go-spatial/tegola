package geom

import (
	"errors"
)

// ErrNilPointMS is thrown when a point is null but shouldn't be
var ErrNilPointMS = errors.New("geom: nil PointMS")

// Point describes a simple 3D point with SRID
type PointMS struct {
	Srid uint32
	Xym  PointM
}

// XYMS returns the struct itself
func (p PointMS) XYMS() struct {
	Srid uint32
	Xym  PointM
} {
	return p
}

// XYM returns 3D+1D point
func (p PointMS) XYM() PointM {
	return p.Xym
}

// S returns the srid as uint32
func (p PointMS) S() uint32 {
	return p.Srid
}

// SetXYMS sets the XYM coordinates and the SRID
func (p *PointMS) SetXYMS(srid uint32, xym PointM) (err error) {
	if p == nil {
		return ErrNilPointMS
	}

	p.Srid = srid
	p.Xym = xym
	return
}
