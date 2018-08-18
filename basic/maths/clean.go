package maths

import (
	"fmt"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/maths"
)

var ErrUnableToClean = fmt.Errorf("Unable to clean MultiPolygon.")

// cleanPolygon will take a look at a polygon and attempt to clean it, returning one or more valid polygons, and an invalid line if it exists.
// It will remove polygons that have no lines, and remove lines that have no points.
// A valid polygon will have the following shape. The first linestring will be clockwise, and all other
// linestrings are counter-clockwise.
func cleanPolygon(p tegola.Polygon) (polygons []basic.Polygon, invalids basic.Polygon) {
	// If the polygon is empty, return empty polygons.
	if p == nil {
		return polygons, invalids
	}
	lines := p.Sublines()
	// If there are no lines, then we return empty polygons.
	if len(lines) == 0 {
		return polygons, invalids
	}
	var currentPolygon basic.Polygon
	for _, l := range lines {
		bl := basic.CloneLine(l)
		// skip lines that don't have any points.
		if len(bl) == 0 {
			continue
		}
		switch bl.Direction() {
		case maths.Clockwise:
			// Need to start a new polygon, after adding the current polygon to the polygons
			// array.
			if currentPolygon != nil {
				polygons = append(polygons, currentPolygon)
				currentPolygon = nil
			}
		case maths.CounterClockwise:
			if currentPolygon == nil {
				// This is an error. There can only be one line that is invalid.
				invalids = append(invalids, bl)
				continue
			}
		}
		currentPolygon = append(currentPolygon, bl)
	}
	if currentPolygon != nil {
		polygons = append(polygons, currentPolygon)
	}
	return polygons, invalids
}

// cleanMultiPolygon will take a look at a multipolygon and attemp to remove, consolidate to turn
// the given multipolygon into a OGC compliant polygon.
func cleanMultiPolygon(mpolygon tegola.MultiPolygon) (mp basic.MultiPolygon, err error) {
	for _, p := range mpolygon.Polygons() {
		poly, invalids := cleanPolygon(p)
		invalidLen := len(invalids)
		mpLen := len(mp)
		switch {
		// This is an error; we can not clean this.
		case invalidLen != 0 && mpLen == 0:
			return mp, ErrUnableToClean
			// We need to add the invalid lines to the last polygon.
		case invalidLen != 0 && mpLen != 0:
			mp[len(mp)-1] = append(mp[len(mp)-1], invalids...)
			continue
		}
		mp = append(mp, poly...)
	}
	return mp, nil
}

func MakeValid(geo tegola.Geometry) (basic.Geometry, error) {
	switch g := geo.(type) {
	/*
		case tegola.Point:
			return basic.Point{g.X(), g.Y()}
		case tegola.LineString:
			return basic.CloneLine(g)
		case tegola.Polygon:
			// We are going to make the polygon OGC valid

				pl, invalids := cleanPolygon(g)
				if len(invalids) > 0 {
					// If there is one or more invalid polygon, we are going to assume that
					// it is the first polygon, and the that the firt line of the polygon is

				}
	*/
	case tegola.MultiPolygon:
		return cleanMultiPolygon(g)
	}
	return basic.Clone(geo), nil
}
