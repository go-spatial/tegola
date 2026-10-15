//go:build cgo

package server

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider/gpkg"
)

// Literal fixture encoders are independent of production geometry codecs.
func crsNative(srid uint32, body []byte) []byte {
	b := []byte{'G', 'P', 0, 1}
	b = binary.LittleEndian.AppendUint32(b, srid)
	return append(b, body...)
}
func crsWKBPositions(kind uint32, positions ...[2]float64) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{1}, kind)
	if kind != 1 {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(positions)))
	}
	for _, position := range positions {
		for _, ordinate := range position {
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(ordinate))
		}
	}
	return b
}
func crsWKBHole() []byte {
	b := binary.LittleEndian.AppendUint32([]byte{1}, 3)
	b = binary.LittleEndian.AppendUint32(b, 2)
	for _, ring := range [][][2]float64{
		{{14, 29}, {16, 29}, {16, 31}, {14, 31}, {14, 29}},
		{{14.8, 29.8}, {14.8, 30.2}, {15.2, 30.2}, {15.2, 29.8}, {14.8, 29.8}},
	} {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(ring)))
		for _, point := range ring {
			for _, ordinate := range point {
				b = binary.LittleEndian.AppendUint64(b, math.Float64bits(ordinate))
			}
		}
	}
	return b
}

func crsSRSDefinition(srid uint32) string {
	base := `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563]],PRIMEM["Greenwich",0],UNIT["degree",0.0174532925199433],AUTHORITY["EPSG","4326"]]`
	switch srid {
	case 4326:
		return base
	case 3857:
		return `PROJCS["WGS 84 / Pseudo-Mercator",` + base + `,PROJECTION["Mercator_1SP"],PARAMETER["central_meridian",0],PARAMETER["scale_factor",1],PARAMETER["false_easting",0],PARAMETER["false_northing",0],UNIT["metre",1],AXIS["Easting",EAST],AXIS["Northing",NORTH],EXTENSION["PROJ4","+proj=merc +a=6378137 +b=6378137 +lat_ts=0 +lon_0=0 +x_0=0 +y_0=0 +k=1 +units=m +nadgrids=@null +wktext +no_defs"],AUTHORITY["EPSG","3857"]]`
	case 32633:
		return `PROJCS["WGS 84 / UTM zone 33N",` + base + `,PROJECTION["Transverse_Mercator"],PARAMETER["latitude_of_origin",0],PARAMETER["central_meridian",15],PARAMETER["scale_factor",0.9996],PARAMETER["false_easting",500000],PARAMETER["false_northing",0],UNIT["metre",1],AXIS["Easting",EAST],AXIS["Northing",NORTH],AUTHORITY["EPSG","32633"]]`
	default:
		panic("unknown literal fixture SRS")
	}
}

func crsGPKG(t *testing.T, srid uint32, point [2]float64, full bool) *gpkg.Provider {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crs-owned.gpkg")
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
		_, err := db.Exec(`CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,n INTEGER,at INTEGER);
CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER);
CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,min_x REAL,min_y REAL,max_x REAL,max_y REAL,srs_id INTEGER);
CREATE TABLE gpkg_spatial_ref_sys(srs_name TEXT NOT NULL,srs_id INTEGER NOT NULL PRIMARY KEY,organization TEXT NOT NULL,organization_coordsys_id INTEGER NOT NULL,definition TEXT NOT NULL,description TEXT);
INSERT INTO gpkg_spatial_ref_sys VALUES('Undefined Cartesian',-1,'NONE',-1,'undefined','undefined Cartesian coordinate reference system'),('Undefined geographic',0,'NONE',0,'undefined','undefined geographic coordinate reference system');`)
		if err != nil {
			t.Fatal(err)
		}
		for _, code := range []uint32{4326, 3857, 32633} {
			if _, err := db.Exec("INSERT INTO gpkg_spatial_ref_sys VALUES(?,?,'EPSG',?,?,?)", strconv.FormatUint(uint64(code), 10), code, code, crsSRSDefinition(code), "canonical fixture CRS"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec("INSERT INTO gpkg_geometry_columns VALUES('items','geom','GEOMETRY',?,0,0);", srid); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO gpkg_contents VALUES('items','features',?,?,?, ?,?)", point[0]-1, point[1]-1, point[0]+1, point[1]+1, srid); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO items VALUES(10,?,'asymmetric',7,0)", crsNative(srid, crsWKBPositions(1, point))); err != nil {
			t.Fatal(err)
		}
		if full {
			rows := []struct {
				id       int
				geometry any
				name     string
			}{
				{20, crsNative(srid, crsWKBPositions(1, [2]float64{16, 31})), "outside"},
				{30, crsNative(srid, crsWKBPositions(2, [2]float64{14, 30}, [2]float64{16, 30})), "boundary"},
				{40, nil, "absence"}, {50, crsNative(srid, crsWKBHole()), "hole"},
			}
			for _, row := range rows {
				if _, err := db.Exec("INSERT INTO items VALUES(?,?,?,7,0)", row.id, row.geometry, row.name); err != nil {
					t.Fatal(err)
				}
			}
		}
	}()
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{{"name": "source", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "gpkg", "srid": int(srid), "fields": []string{"name", "n"}, "temporal_field": "at", "temporal_storage": "unix_seconds"}}}, nil)
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

func TestFeatureCRSRealGPKGQueryFrameAndPaging(t *testing.T) {
	p := crsGPKG(t, 4326, [2]float64{15, 30}, true)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	srv := protocolServer(t, p, layers[0], "/")
	base := "/features/collections/public/items"
	// Each rectangle has its own fixed oracle. No cross-projection ID equality
	// assertion is used. The polygon's source envelope covers the query, but its
	// hole must reject it before page/count; the line crosses the rectangle.
	for _, tc := range []struct{ uri, bbox string }{{features.CRS84, "15,30,15,30"}, {"http://www.opengis.net/def/crs/EPSG/0/4326", "30,15,30,15"}, {"http://www.opengis.net/def/crs/EPSG/0/3857", "1669692.3618991037,3503449.843504374,1669892.3618991037,3503649.843504374"}} {
		for offset, want := range []uint64{10, 30, 40} {
			q := url.Values{"bbox": {tc.bbox}, "bbox-crs": {tc.uri}, "crs": {"http://www.opengis.net/def/crs/EPSG/0/3857"}, "limit": {"1"}, "offset": {strconv.Itoa(offset)}, "filter": {"n = 7"}, "datetime": {"1970-01-01T00:00:00Z"}}
			r := protocolRequest(t, srv, "GET", base+"?"+q.Encode())
			if r.status != 200 || !slices.Equal(protocolIDs(t, r), []uint64{want}) {
				t.Fatalf("query-frame%s offset%d status%d IDs%v", tc.uri, offset, r.status, protocolIDs(t, r))
			}
			if r.header.Get("Content-Crs") != "<http://www.opengis.net/def/crs/EPSG/0/3857>" {
				t.Fatal("output CRS coupled to bbox CRS")
			}
			var page struct {
				NumberMatched *uint64
				Links         []struct{ Rel, Href string }
			}
			if err := json.Unmarshal(r.body, &page); err != nil {
				t.Fatal(err)
			}
			if offset < 2 && page.NumberMatched != nil || offset == 2 && (page.NumberMatched == nil || *page.NumberMatched != 3) {
				t.Fatal("query-frame paging count changed")
			}
			next := false
			for _, link := range page.Links {
				if link.Rel == "next" {
					next = true
					u, err := url.Parse(link.Href)
					if err != nil || u.Query().Get("bbox-crs") != tc.uri || u.Query().Get("crs") != q.Get("crs") {
						t.Fatal("CRS paging link lost query/output frame")
					}
				}
			}
			if next != (offset < 2) {
				t.Fatal("query-frame lookahead changed")
			}
			if want == 10 {
				assertCRSPosition(t, crsPoint(t, r, false), []float64{1669792.3618991037, 3503549.843504374}, .15)
			}
		}
	}
}

func TestFeatureCRSRealGPKGProjectedStorage(t *testing.T) {
	for _, tc := range []struct {
		srid   uint32
		source [2]float64
		uri    string
	}{{3857, [2]float64{1669792.3618991037, 3503549.843504374}, "http://www.opengis.net/def/crs/EPSG/0/3857"}, {32633, [2]float64{500000, 3318785.352608442}, "http://www.opengis.net/def/crs/EPSG/0/32633"}} {
		p := crsGPKG(t, tc.srid, tc.source, false)
		layers, err := p.Layers()
		if err != nil {
			t.Fatal(err)
		}
		srv := protocolServer(t, p, layers[0], "/")
		for _, item := range []bool{false, true} {
			path := "/features/collections/public/items"
			if item {
				path += "/10"
			}
			r := protocolRequest(t, srv, "GET", path)
			if r.status != 200 || r.header.Get("Content-Crs") != "<"+features.CRS84+">" {
				t.Fatal("projected storage default output unavailable")
			}
			assertCRSPosition(t, crsPoint(t, r, item), []float64{15, 30}, 1e-6)
		}
		r := protocolRequest(t, srv, "GET", "/features/collections/public/items/10?crs="+url.QueryEscape(tc.uri))
		if r.status != 200 {
			t.Fatal("advertised projected storage CRS rejected")
		}
		assertCRSPosition(t, crsPoint(t, r, true), tc.source[:], .15)
	}
}

func crsXYZWire(kind uint32, points ...[3]float64) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{1}, kind+1000)
	if kind != 1 {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(points)))
	}
	for _, point := range points {
		for _, ordinate := range point {
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(ordinate))
		}
	}
	return b
}

func TestFeatureCRSRealGPKGHeightAndMixedQueryFrame(t *testing.T) {
	for _, dimension := range []string{"xyz", "mixed_xy_xyz"} {
		t.Run(dimension, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "height-owned.gpkg")
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
				if _, err := db.Exec(`CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT); CREATE TABLE gpkg_spatial_ref_sys(srs_name TEXT,srs_id INTEGER PRIMARY KEY,organization TEXT,organization_coordsys_id INTEGER,definition TEXT,description TEXT);`); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec("INSERT INTO gpkg_spatial_ref_sys VALUES('WGS84',4326,'EPSG',4326,?,'explicit canonical horizontal source')", crsSRSDefinition(4326)); err != nil {
					t.Fatal(err)
				}
				rows := []struct {
					id   int
					wire []byte
				}{
					{10, crsXYZWire(1, [3]float64{15, 30, 23})},
					{20, crsXYZWire(1, [3]float64{15, 30, 99})},
					{30, crsXYZWire(2, [3]float64{14, 30, 22}, [3]float64{16, 30, 24})},
					// XY and Z extents independently overlap the query, but the
					// only Z=23 position is at longitude14 outside its rectangle.
					{35, crsXYZWire(2, [3]float64{14, 30, 23}, [3]float64{16, 30, 25})},
					{40, nil},
				}
				if dimension == "mixed_xy_xyz" {
					collection := binary.LittleEndian.AppendUint32([]byte{1}, 7)
					collection = binary.LittleEndian.AppendUint32(collection, 2)
					collection = append(collection, crsWKBPositions(1, [2]float64{15, 30})...)
					collection = append(collection, crsXYZWire(1, [3]float64{15, 30, 23})...)
					rows = append(rows, struct {
						id   int
						wire []byte
					}{50, collection})
				}
				for _, row := range rows {
					var wire any
					if row.wire != nil {
						wire = row.wire
					}
					if _, err := db.Exec("INSERT INTO items VALUES(?,?,'literal')", row.id, wire); err != nil {
						t.Fatal(err)
					}
				}
			}()
			tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{{"name": "source", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkb", "srid": 4326, "spatial_dimension": dimension, "vertical_crs": features.CRS84h, "fields": []string{"name"}}}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			p := tiler.(*gpkg.Provider)
			t.Cleanup(func() {
				if err := p.Close(); err != nil {
					t.Error(err)
				}
			})
			layers, err := p.Layers()
			if err != nil {
				t.Fatal(err)
			}
			srv := protocolServer(t, p, layers[0], "/")
			base := "/features/collections/public/items"
			r := protocolRequest(t, srv, "GET", base+"/10?crs="+url.QueryEscape("http://www.opengis.net/def/crs/EPSG/0/4979"))
			if r.status != 200 {
				t.Fatalf("height output status%d body%s", r.status, r.body)
			}
			assertCRSPosition(t, crsPoint(t, r, true), []float64{30, 15, 23}, 1e-6)
			metadata := protocolRequest(t, srv, "GET", "/features/collections/public")
			var catalog struct {
				CRS []string `json:"crs"`
			}
			if err := json.Unmarshal(metadata.body, &catalog); err != nil {
				t.Fatal(err)
			}
			mercator, err := features.ResolveCRS("http://www.opengis.net/def/crs/EPSG/0/3857")
			if err != nil {
				t.Fatal(err)
			}
			// The descriptor constructor identifies the advertised application CRS;
			// coordinates and memberships remain independent literal oracles.
			heightMercator, err := features.NewApplicationCRS(mercator.Definition().Definition, 3857, 3)
			if err != nil {
				t.Fatal(err)
			}
			mercatorURI := heightMercator.URI()
			if !slices.Contains(catalog.CRS, mercatorURI) {
				t.Fatal("advertised height-preserving Mercator target missing")
			}
			r = protocolRequest(t, srv, "GET", base+"/10?crs="+url.QueryEscape(mercatorURI))
			if r.status != 200 {
				t.Fatal("application height output rejected")
			}
			assertCRSPosition(t, crsPoint(t, r, true), []float64{1669792.3618991037, 3503549.843504374, 23}, .15)
			if crsPoint(t, r, true)[2] != 23 {
				t.Fatal("ellipsoidal height changed")
			}
			want := []uint64{10, 30, 40}
			if dimension == "mixed_xy_xyz" {
				want = append(want, 50)
			}
			for offset, id := range want {
				q := url.Values{"bbox": {"1669692.3618991037,3503449.843504374,23,1669892.3618991037,3503649.843504374,23"}, "bbox-crs": {mercatorURI}, "crs": {mercatorURI}, "limit": {"1"}, "offset": {strconv.Itoa(offset)}}
				r = protocolRequest(t, srv, "GET", base+"?"+q.Encode())
				if r.status != 200 || !slices.Equal(protocolIDs(t, r), []uint64{id}) {
					t.Fatalf("exact correlated6D page offset%d status%d body%s", offset, r.status, r.body)
				}
				var page struct {
					NumberMatched *uint64 `json:"numberMatched"`
					Links         []struct {
						Rel string `json:"rel"`
					} `json:"links"`
				}
				if err := json.Unmarshal(r.body, &page); err != nil {
					t.Fatal(err)
				}
				next := false
				for _, link := range page.Links {
					next = next || link.Rel == "next"
				}
				if next != (offset < len(want)-1) {
					t.Fatal("3D exact lookahead changed")
				}
				if next {
					if page.NumberMatched != nil {
						t.Fatal("early3D page falsely claimed exact count")
					}
				} else if page.NumberMatched == nil || *page.NumberMatched != uint64(len(want)) {
					t.Fatal("exhausted3D count differs from static fixture")
				}
			}
			if dimension == "mixed_xy_xyz" {
				r = protocolRequest(t, srv, "GET", base+"/50?crs="+url.QueryEscape("http://www.opengis.net/def/crs/EPSG/0/4979"))
				var body struct {
					Geometry struct {
						Type     string `json:"type"`
						Children []struct {
							Type        string    `json:"type"`
							Coordinates []float64 `json:"coordinates"`
						} `json:"geometries"`
					} `json:"geometry"`
				}
				if r.status != 200 || json.Unmarshal(r.body, &body) != nil || body.Geometry.Type != "GeometryCollection" || len(body.Geometry.Children) != 2 {
					t.Fatal("mixed collection structure changed")
				}
				for i, want := range [][]float64{{30, 15}, {30, 15, 23}} {
					if body.Geometry.Children[i].Type != "Point" {
						t.Fatal("mixed child type changed")
					}
					assertCRSPosition(t, body.Geometry.Children[i].Coordinates, want, 1e-6)
				}
			}
		})
	}
}
