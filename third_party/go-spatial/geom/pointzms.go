package geom

import (
	"errors"
)

// ErrNilPointZMS is thrown when a point is null but shouldn't be
var ErrNilPointZMS = errors.New("geom: nil PointZMS")

// Point describes a simple 3D+1D point with SRID
type PointZMS struct {
	Srid uint32
	Xyzm PointZM
}

// XYZMS returns the struct itself
func (p PointZMS) XYZMS() struct {
	Srid uint32
	Xyzm PointZM
} {
	return p
}

// XYZM returns 3D+1D point
func (p PointZMS) XYZM() PointZM {
	return p.Xyzm
}

// S returns the srid as uint32
func (p PointZMS) S() uint32 {
	return p.Srid
}

// SetXYZMS sets the XYZM coordinates and the SRID
func (p *PointZMS) SetXYZMS(srid uint32, xyzm PointZM) (err error) {
	if p == nil {
		return ErrNilPointZMS
	}

	p.Srid = srid
	p.Xyzm = xyzm
	return
}
