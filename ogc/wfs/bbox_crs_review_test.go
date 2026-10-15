package wfs

import (
	"context"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"testing"
)

const bboxCustomDefinition = "+proj=etmerc +ellps=bessel +towgs84=1,2,3,0,0,0,0 +lon_0=9 +lat_0=50 +k_0=1 +x_0=0 +y_0=0 +units=m +no_defs"

type bboxCRSSource struct {
	reviewQuerySource
	queries []provider.FeatureQuery
}

func (*bboxCRSSource) SRID() uint64 { return 340000001 }
func (*bboxCRSSource) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	return provider.FeatureCRSDefinition{HorizontalSRID: 340000001, Definition: bboxCustomDefinition, Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}, nil
}
func (s *bboxCRSSource) QueryFeatures(_ context.Context, _ string, q provider.FeatureQuery, _ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	s.queries = append(s.queries, q)
	return provider.FeatureQueryResult{}, nil
}
func TestNativeBBoxCRSReview(t *testing.T) {
	src := &bboxCRSSource{}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: src, Querier: src}})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := svc.CollectionCRS("sites")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []Version{V110, V200, V202} {
		req, ex := ParseGetFeatureKVP(v, map[string]string{"typenames": "sites", "bbox": "-2000,1000,5000,8000," + catalog.StorageURI()})
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		_, ex = ExecuteGetFeature(context.Background(), svc, req)
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		q := src.queries[len(src.queries)-1]
		if q.BoundsSRID != 340000001 || q.BoundsCRSDefinition != bboxCustomDefinition || len(q.Bounds) != 1 || q.Bounds[0] != (geom.Extent{-2000, 1000, 5000, 8000}) {
			t.Fatalf("lost native bbox identity: %+v", q)
		}
	}
}
func TestBBoxCRSCompatibilityReview(t *testing.T) {
	src := &bboxCRSSource{}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: src, Querier: src}})
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := svc.CollectionCRS("sites")
	for _, v := range []Version{V110, V200, V202} {
		for _, tc := range []struct {
			raw  string
			want geom.Extent
		}{
			{"10,20,30,40", geom.Extent{10, 20, 30, 40}},
			{"10,20,30,40,http://www.opengis.net/def/crs/OGC/1.3/CRS84", geom.Extent{10, 20, 30, 40}},
			{"20,10,40,30,http://www.opengis.net/def/crs/EPSG/0/4326", geom.Extent{10, 20, 30, 40}},
		} {
			req, ex := ParseGetFeatureKVP(v, map[string]string{"typenames": "sites", "bbox": tc.raw})
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			_, ex = ExecuteGetFeature(context.Background(), svc, req)
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			q := src.queries[len(src.queries)-1]
			if q.BoundsSRID != 4326 || q.Bounds[0] != tc.want {
				t.Fatalf("axis compatibility: %+v", q)
			}
			if tc.raw == "10,20,30,40" && q.BoundsCRSDefinition != "" {
				t.Fatal("four-ordinate semantics changed")
			}
		}
	}
	for _, raw := range []string{"0,0,1,1,", "0,0,1,1,urn:unknown", "0,0,1,1,urn:uuid:00000000-0000-0000-0000-000000000000", "0,0,1,1,http://www.opengis.net/def/crs/EPSG/0/4979", "0,0,1,1,extra,extra", "NaN,0,1,1," + catalog.StorageURI(), "3,0,1,1," + catalog.StorageURI()} {
		before := len(src.queries)
		req, ex := ParseGetFeatureKVP(V200, map[string]string{"typenames": "sites", "bbox": raw})
		if len(ex) == 0 {
			_, ex = ExecuteGetFeature(context.Background(), svc, req)
		}
		if len(ex) == 0 || len(src.queries) != before {
			t.Fatalf("invalid CRS reached provider: %s", raw)
		}
	}
	for _, v := range []Version{V200, V202} {
		req, ex := ParseGetPropertyValueKVP(v, map[string]string{"typenames": "sites", "valuereference": "rank", "bbox": "-2,1,5,8," + catalog.StorageURI()})
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		_, ex = ExecuteGetPropertyValue(context.Background(), svc, req)
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		q := src.queries[len(src.queries)-1]
		if q.BoundsCRSDefinition != bboxCustomDefinition || q.Bounds[0] != (geom.Extent{-2, 1, 5, 8}) {
			t.Fatal("GPV lost native CRS")
		}
	}
}
