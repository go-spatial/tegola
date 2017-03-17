package maths

/*
// cleanPolygon will take a look at a polygon and attempt to clean it, returning one or more valid polygons, and an invalid line if it exists.
// It will remove polygons that have no lines, and remove lines that have no points.
// A valid polygon will have the following shape. The first linestring will be clockwise, and all
// linestrings are counter-clockwise.
func cleanPolygon(p tegola.Polygon) (polygons []basic.Polygon, invalid basic.Polygon) {
	// If the polygon is empty, return empty polygons.
	if p == nil {
		return polygons, invalids
	}
	lines := p.Lines()
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
		case Clockwise:
			// Need to start a new polygon, after adding the current polygon to the polygons
			// array.
			if currentPolygon != nil {
				polygons = append(polygons, currentPolygon)
				currentPolygon = nil
			}
		case CounterClockwise:
			if currentPolygon == nil {
				// This is an error. There can only be one line that is invalid.
				invalid = append(invalid, bl)
				continue
			}
		}
		currentPolygon = append(currentPolygon, bl)
	}
	if currentPolygon != nil {
		polygons = append(polygons, currentPolygon)
	}
	return polygons, invalid
}

func cleanMultiPolygon(mpolygon tegola.MultiPolygon) (mp basic.MultiPolygon, err error) {
	for _, p := range mpolygon.Polygons() {
		poly, invalid := cleanPolygon(p)
		invalidLen = len(invalid)
		mpLen = len(mp)
		switch {
		// This is an error; we can not clean this.
		case invalidLen != 0 && mpLen == 0:
			return mp, fmt.Errorf("Unable to clean MultiPolygon.")
		case invalidLen != 0 && mpLen != 0:
			lastPoly := mp[len(mp)-1]
			lastPoly = append(lastPoly, invalid.Lines()...)
			continue
		}
		mp = append(mp, poly...)
	}
	return mp, nil
}

*/
