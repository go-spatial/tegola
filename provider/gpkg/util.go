package gpkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/terranodo/tegola/maths/points"
)

const (
	bboxToken = "!BBOX!"
	zoomToken = "!ZOOM!"
)

// TODO(arolek): replace points.BoundingBox with geom.Extent
func replaceTokens(qtext string, zoom uint64, extent points.BoundingBox) string {
	// --- Convert tokens provided to SQL
	// The ZOOM token requires two parameters, both filled with the current zoom level.
	//	Until support for named parameters, the ZOOM token must follow the BBOX token.
	/*
		tokensPresent := make(map[string]bool)

		if strings.Count(qtext, "!BBOX!") > 0 {
			tokensPresent["BBOX"] = true
			qtext = strings.Replace(qtext, "!BBOX!", "minx <= ? AND maxx >= ? AND miny <= ? AND maxy >= ?", 1)
		}

		if strings.Count(qtext, "!ZOOM!") > 0 {
			tokensPresent["ZOOM"] = true
			qtext = strings.Replace(qtext, "!ZOOM!", "min_zoom <= ? AND max_zoom >= ?", 1)
		}
	*/

	tokenReplacer := strings.NewReplacer(
		// The BBOX token requires parameters ordered as [maxx, minx, maxy, miny] and checks for overlap.
		// 	Until support for named parameters, we'll only support one BBOX token per query.
		bboxToken, fmt.Sprintf("minx <= %v AND maxx >= %v AND miny <= %v AND maxy >= %v", extent[2], extent[0], extent[3], extent[1]),
		zoomToken, strconv.FormatUint(zoom, 10),
	)

	return tokenReplacer.Replace(qtext)
}
