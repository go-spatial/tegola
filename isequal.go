package tegola

// IsPointEqual will check to see if the two tegola points are equal.
func IsPointEqual(p1, p2 Point) bool {
	if p1 == nil || p2 == nil {
		return p1 == p2
	}
	return p1.X() == p2.X() && p1.Y() == p2.Y()
}

// IsPoint3Equal will check to see if the two 3d tegola points are equal.
func IsPoint3Equal(p1, p2 Point3) bool {
	return p1.X() == p2.X() && p1.Y() == p2.Y() && p1.Z() == p2.Z()
}

// IsMultiPointEqual will check to see if the two provided multipoints are equal
func IsMultiPointEqual(mp1, mp2 MultiPoint) bool {
	pts1, pts2 := mp1.Points(), mp2.Points()
	if len(pts1) != len(pts2) {
		return false
	}
	for i, pt := range pts1 {
		if !IsPointEqual(pt, pts2[i]) {
			return false
		}
	}
	return true
}

// IsLineStringEqual will check to see if the two linesstrings provided are equal.
func IsLineStringEqual(l1, l2 LineString) bool {
	pts1, pts2 := l1.Subpoints(), l2.Subpoints()
	if len(pts1) != len(pts2) {
		return false
	}
	for i, pt := range pts1 {
		if !IsPointEqual(pt, pts2[i]) {
			return false
		}
	}
	return true
}

// IsMultiLineEqual will check to see if the two Multilines that are provided are equal.
func IsMultiLineEqual(ml1, ml2 MultiLine) bool {
	lns1, lns2 := ml1.Lines(), ml2.Lines()
	if len(lns1) != len(lns2) {
		return false
	}
	for i, ln := range lns1 {
		if !IsLineStringEqual(ln, lns2[i]) {
			return false
		}
	}
	return true
}

// PolygonIsEqual will check to see if the two provided polygons are equal.
func IsPolygonEqual(p1, p2 Polygon) bool {
	lns1, lns2 := p1.Sublines(), p2.Sublines()
	if len(lns1) != len(lns2) {
		return false
	}
	for i, ln := range lns1 {
		if !IsLineStringEqual(ln, lns2[i]) {
			return false
		}
	}
	return true
}

// MultiPolygonIsEqual will check to see if the two provided multi-polygons are equal.
func IsMultiPolygonEqual(mp1, mp2 MultiPolygon) bool {
	pgs1, pgs2 := mp1.Polygons(), mp2.Polygons()
	if len(pgs1) != len(pgs2) {
		return false
	}
	for i, pg := range pgs1 {
		if !IsPolygonEqual(pg, pgs2[i]) {
			return false
		}
	}
	return true
}

// GeometryIsEqual will check to see if the two given geometeries are equal. This function does not check to see if there are any
// recursive structures if there are any recursive structures it will hang. If the type of the geometry is unknown, it is assumed
// that it does not match any other geometries.
func IsGeometryEqual(g1, g2 Geometry) bool {
	switch geo1 := g1.(type) {
	case Point:
		geo2, ok := g2.(Point)
		if !ok {
			return false
		}
		return IsPointEqual(geo1, geo2)
	case Point3:
		geo2, ok := g2.(Point3)
		if !ok {
			return false
		}
		return IsPoint3Equal(geo1, geo2)
	case MultiPoint:
		geo2, ok := g2.(MultiPoint)
		if !ok {
			return false
		}
		return IsMultiPointEqual(geo1, geo2)
	case LineString:
		geo2, ok := g2.(LineString)
		if !ok {
			return false
		}
		return IsLineStringEqual(geo1, geo2)
	case MultiLine:
		geo2, ok := g2.(MultiLine)
		if !ok {
			return false
		}
		return IsMultiLineEqual(geo1, geo2)
	case Polygon:
		geo2, ok := g2.(Polygon)
		if !ok {
			return false
		}
		return IsPolygonEqual(geo1, geo2)
	case MultiPolygon:
		geo2, ok := g2.(MultiPolygon)
		if !ok {
			return false
		}
		return IsMultiPolygonEqual(geo1, geo2)
	case Collection:
		geo2, ok := g2.(Collection)
		if !ok {
			return false
		}
		return IsCollectionEqual(geo1, geo2)
	}
	// If we don't know the type, we will assume they don't match.
	return false
}

// CollectionIsEqual will check to see if the provided collections are equal. This function does not check to see if the collections
// contain any recursive structures, and if there are any recursive structures it will hang. If the collections contains any unknown
// geometries it will be assumed to not match.
func IsCollectionEqual(c1, c2 Collection) bool {
	geos1, geos2 := c1.Geometries(), c2.Geometries()
	if len(geos1) != len(geos2) {
		return false
	}
	for i, geo := range geos1 {
		if !IsGeometryEqual(geo, geos2[i]) {
			return false
		}
	}
	return true
}
