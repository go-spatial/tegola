package gpkg

import "strings"

func replaceTokens(qtext string) (string, map[string]bool) {
	// --- Convert tokens provided to SQL
	// The BBOX token requires parameters ordered as [maxx, minx, maxy, miny] and checks for overlap.
	// 	Until support for named parameters, we'll only support one BBOX token per query.
	// The ZOOM token requires two parameters, both filled with the current zoom level.
	//	Until support for named parameters, the ZOOM token must follow the BBOX token.

	tokensPresent := make(map[string]bool)

	if strings.Count(qtext, "!BBOX!") > 0 {
		tokensPresent["BBOX"] = true
		qtext = strings.Replace(qtext, "!BBOX!", "minx <= ? AND maxx >= ? AND miny <= ? AND maxy >= ?", 1)
	}

	if strings.Count(qtext, "!ZOOM!") > 0 {
		tokensPresent["ZOOM"] = true
		qtext = strings.Replace(qtext, "!ZOOM!", "min_zoom <= ? AND max_zoom >= ?", 1)
	}

	return qtext, tokensPresent
}
