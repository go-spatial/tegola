//go:build cgo

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider/gpkg"
)

func filterProtocolGPKG(t *testing.T) *gpkg.Provider {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filter-protocol.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}()
		_, err := db.Exec(`CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,n INTEGER,s TEXT,b BOOLEAN,at INTEGER,minx REAL,miny REAL,maxx REAL,maxy REAL);
INSERT INTO items VALUES
(10,'POINT(15 30)',7,'Alpha',1,0,15,30,15,30),
(20,'POINT(16 31)',7,'Alpha',0,1,16,31,16,31),
(30,'POINT(100 50)',7,NULL,1,0,100,50,100,50),
(40,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL),
(50,'POINT(15 30)',0,'x'' OR TRUE -- ;',0,0,15,30,15,30),
(99,'source-integrity-sentinel',999,'hidden',0,0,15,30,15,30);`)
		if err != nil {
			t.Fatal(err)
		}
	}()
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{{
		"name": "source", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom",
		"geometry_format": "wkt", "geometry_type": "point", "srid": 4326,
		"fields": []string{"n", "s", "b"}, "temporal_field": "at", "temporal_storage": "unix_seconds",
		"bbox_minx_fieldname": "minx", "bbox_miny_fieldname": "miny", "bbox_maxx_fieldname": "maxx", "bbox_maxy_fieldname": "maxy",
	}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*gpkg.Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	return p
}

func TestFeatureFilterProtocolRealGPKG(t *testing.T) {
	p := filterProtocolGPKG(t)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	srv := protocolServer(t, p, layers[0], "/")
	base := "/features/collections/public/items"
	for _, tc := range []struct {
		name, expression, extra string
		ids                     []uint64
		count                   uint64
		countKnown, hasMore     bool
	}{
		{"first page", "n = 7", "&limit=1", []uint64{10}, 3, false, true},
		{"second page", "n = 7", "&limit=1&offset=1", []uint64{20}, 3, false, true},
		{"last page", "n = 7", "&limit=1&offset=2", []uint64{30}, 3, true, false},
		{"exact fraction", "n = 7.1", "", nil, 0, true, false},
		{"NULL under NOT", "NOT(n = 7) AND n < 100", "", []uint64{50}, 1, true, false},
		{"absence", "n IS NULL", "&bbox=0,0,1,1", []uint64{40}, 1, true, false},
		{"literal injection", "s = 'x'' OR TRUE -- ;'", "", []uint64{50}, 1, true, false},
		{"bbox and datetime", "n = 7", "&bbox=14,29,17,32&datetime=1970-01-01T00:00:00Z", []uint64{10}, 1, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := protocolRequest(t, srv, "GET", base+"?filter="+url.QueryEscape(tc.expression)+tc.extra)
			// This corrupt row is in spatial/time range but excluded by the bound
			// filter. Successful output proves it was not decoded before filtering.
			if r.status != 200 || !slices.Equal(protocolIDs(t, r), tc.ids) {
				t.Fatalf("status=%d IDs=%v expected=%v", r.status, protocolIDs(t, r), tc.ids)
			}
			var body struct {
				NumberMatched *uint64 `json:"numberMatched"`
				Features      []struct {
					ID         uint64
					Properties map[string]any
				} `json:"features"`
				Links []struct{ Rel string }
			}
			if err := json.Unmarshal(r.body, &body); err != nil {
				t.Fatal(err)
			}
			if tc.countKnown && (body.NumberMatched == nil || *body.NumberMatched != tc.count) || !tc.countKnown && body.NumberMatched != nil {
				t.Fatal("count differs from fixed fixture membership")
			}
			// HasMore is exposed as a next link, not a GeoJSON member.
			next := false
			for _, link := range body.Links {
				next = next || link.Rel == "next"
			}
			if next != tc.hasMore {
				t.Fatal("lookahead next link differs from static paging expectation")
			}
			for _, feature := range body.Features {
				if len(feature.Properties) != 3 {
					t.Fatal("private fields leaked or explicit public NULL omitted")
				}
				for _, name := range []string{"b", "n", "s"} {
					if _, ok := feature.Properties[name]; !ok {
						t.Fatal("selected public property missing")
					}
					if feature.ID == 40 && feature.Properties[name] != nil {
						t.Fatal("SQL NULL replaced with a fabricated value")
					}
				}
				if feature.ID == 30 && feature.Properties["s"] != nil {
					t.Fatal("nullable string replaced with a fabricated value")
				}
			}
		})
	}
	// Independently prove the source sentinel is actually corrupt; successful
	// filtered requests above cannot be explained by a tolerant geometry decoder.
	if r := protocolRequest(t, srv, "GET", base+"/99"); r.status != 500 {
		t.Fatal("corrupt source sentinel was not rejected")
	}
	r := protocolRequest(t, srv, "GET", base+"?filter="+url.QueryEscape("n = 7")+"&limit=1")
	var page struct{ Links []struct{ Rel, Href string } }
	if err := json.Unmarshal(r.body, &page); err != nil {
		t.Fatal(err)
	}
	followed := false
	for _, link := range page.Links {
		if link.Rel == "next" {
			parsed, err := url.Parse(link.Href)
			if err != nil || parsed.Query().Get("filter") != "n = 7" {
				t.Fatal("next link lost original filter")
			}
			if next := protocolRequest(t, srv, "GET", parsed.RequestURI()); next.status != 200 || !slices.Equal(protocolIDs(t, next), []uint64{20}) {
				t.Fatal("following advertised page changed static membership")
			}
			followed = true
		}
	}
	if !followed {
		t.Fatal("next page link absent")
	}
	if schema := protocolRequest(t, srv, "GET", "/features/collections/public/queryables", "application/schema+json"); schema.status != 200 {
		t.Fatal("real GPKG queryables unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", base+"?filter=TRUE", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(recorder, req)
	if recorder.Code != 408 {
		t.Fatal("canceled real-provider router query was not stopped")
	}
}
