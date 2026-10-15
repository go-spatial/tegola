package features

import "sort"

const (
	ConformanceCore    = "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core"
	ConformanceGeoJSON = "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson"
	ConformanceHTML    = "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/html"
	ConformanceOpenAPI = "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/oas30"
	ConformanceCRS     = "http://www.opengis.net/spec/ogcapi-features-2/1.0/conf/crs"
	// Reserved vocabulary only: enabling writes does not establish this class.
	// Independent conformance evidence is required before publication.
	ConformancePart4 = "http://www.opengis.net/spec/ogcapi-features-4/1.0/conf/create-replace-delete"
)

// ConformanceClasses describes the shipped FeatureAPI HTTP implementation for
// this publication. It is not a declaration for an arbitrary standalone host of
// Service. The admitted implementation classes have no runtime configuration or
// report-file switches; optional classes require every frozen collection.
func (s *Service) ConformanceClasses() []string {
	classes := make([]string, 0, 5)
	if s == nil || len(s.collections) == 0 {
		return classes
	}
	// GeoJSON, HTML, OpenAPI and Part 2 each depend on Core. Filtering/CQL2
	// classes are deliberately absent from the admitted implementation registry.
	classes = append(classes, ConformanceCore, ConformanceGeoJSON, ConformanceHTML, ConformanceOpenAPI)
	if s.allCollectionsSupportCRS() {
		classes = append(classes, ConformanceCRS)
	}
	sort.Strings(classes)
	return classes
}

func (s *Service) allCollectionsSupportCRS() bool {
	for _, collection := range s.collections {
		if !collection.crsAvailable || collection.sourceProjection == nil || len(collection.crs.URIs()) == 0 {
			return false
		}
		if _, err := collection.crs.ValidateOutput(""); err != nil {
			return false
		}
	}
	return true
}
