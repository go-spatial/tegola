package geom

import "errors"

// ErrNilMultiPolygon is thrown when a MultiPolygon is nul but shouldn't be
var ErrNilMultiPolygon = errors.New("geom: nil MultiPolygon")

// MultiPolygon is a geometry of multiple polygons.
type MultiPolygon [][][][2]float64

// Polygons returns the array of polygons.
func (mp MultiPolygon) Polygons() [][][][2]float64 {
	return mp
}

// SetPolygons modifies the array of 2D coordinates
func (mp *MultiPolygon) SetPolygons(input [][][][2]float64) (err error) {
	if mp == nil {
		return ErrNilMultiPolygon
	}

	*mp = append((*mp)[:0], input...)
	return
}

// AsSegments return a set of []Line
func (mp MultiPolygon) AsSegments() (segs [][][]Line, err error) {
	if len(mp) == 0 {
		return nil, nil
	}
	segs = make([][][]Line, 0, len(mp))
	for i := range mp {
		p := Polygon(mp[i])
		seg, err := p.AsSegments()
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
	}
	return segs, nil
}
