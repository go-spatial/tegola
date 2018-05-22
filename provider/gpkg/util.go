package gpkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-spatial/geom"
)

const (
	bboxToken = "!BBOX!"
	zoomToken = "!ZOOM!"
)

func replaceTokens(qtext string, zoom *uint, extent geom.MinMaxer) string {
	// --- Convert tokens provided to SQL
	// The ZOOM token requires two parameters, both filled with the current zoom level.
	// Until support for named parameters, the ZOOM token must follow the BBOX token.
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
	// Replacement Pairs
	rps := []string{}
	if extent != nil {
		rps = append(rps, bboxToken)
		rps = append(rps,
			fmt.Sprintf("minx <= %v AND maxx >= %v AND miny <= %v AND maxy >= %v", extent.MaxX(), extent.MinX(), extent.MaxY(), extent.MinY()),
		)
	} else {

	}

	if zoom != nil {
		rps = append(rps, zoomToken)
		rps = append(rps, strconv.FormatUint(uint64(*zoom), 10))
	}

	tokenReplacer := strings.NewReplacer(rps...)

	return tokenReplacer.Replace(qtext)
}
