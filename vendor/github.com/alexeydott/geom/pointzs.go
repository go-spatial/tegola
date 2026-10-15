package geom

import (
	"errors"
)

// ErrNilPointZS is thrown when a point is null but shouldn't be
var ErrNilPointZS = errors.New("geom: nil PointZS")

// Point describes a simple 3D point with SRID
type PointZS struct {
	Srid uint32
	Xyz  PointZ
}

// XYZS returns the struct itself
func (p PointZS) XYZS() struct {
	Srid uint32
	Xyz  PointZ
} {
	return p
}

// XYZ returns 3D point
func (p PointZS) XYZ() PointZ {
	return p.Xyz
}

// S returns the srid as uint32
func (p PointZS) S() uint32 {
	return p.Srid
}

// SetXYZS sets the XYZ coordinates and the SRID
func (p *PointZS) SetXYZS(srid uint32, xyz PointZ) (err error) {
	if p == nil {
		return ErrNilPointZS
	}

	p.Srid = srid
	p.Xyz = xyz
	return
}
