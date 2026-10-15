package wfs

import (
	"context"
	"github.com/alexeydott/tegola/ogc/features"
	"strings"
	"testing"
)

func TestPropertyReviewSelectors(t *testing.T) {
	for _, q := range []map[string]string{
		{"typenames": "sites", "valuereference": "rank", "featureid": "other.1"},
		{"typenames": "sites", "valuereference": "rank/unsupported()"},
		{"typenames": "sites", "valuereference": "rank", "filter": "<Filter><Unknown/></Filter>"},
	} {
		if _, ex := ParseGetPropertyValueKVP(V200, q); len(ex) == 0 {
			t.Fatalf("accepted %v", q)
		}
	}
}
func TestPropertyReviewSharedPaging(t *testing.T) {
	src := reviewQuerySource{}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: src, Querier: src}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		q            map[string]string
		want, absent string
	}{
		{map[string]string{"typenames": "sites", "valuereference": "rank", "count": "1", "startindex": "1", "sortby": "rank A"}, ">20<", ">30<"},
		{map[string]string{"typenames": "sites", "valuereference": "rank", "count": "1", "resulttype": "hits"}, `numberMatched="3"`, `numberMatched="1"`},
		{map[string]string{"typenames": "sites", "valuereference": "rank", "count": "0"}, `numberReturned="0"`, "<wfs:member>"},
		{map[string]string{"typenames": "sites", "valuereference": "geometry", "count": "1"}, "<gml:Point", `{&quot;type&quot;`},
	} {
		req, ex := ParseGetPropertyValueKVP(V200, tc.q)
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		out, ex := ExecuteGetPropertyValue(context.Background(), svc, req)
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		if !strings.Contains(out, tc.want) || strings.Contains(out, tc.absent) {
			t.Fatalf("unexpected output: %s", out)
		}
	}
}
func TestPropertyReviewStoredQueryIDs(t *testing.T) {
	sq, _ := GetStoredQuery("urn:ogc:def:query:OGC-WFS::GetFeatureById")
	for _, id := range []string{"other.1", "1garbage", "-1"} {
		if _, ex := sq.ToGetFeature(map[string]string{"typename": "sites", "id": id}); len(ex) == 0 {
			t.Errorf("accepted %s", id)
		}
	}
}
