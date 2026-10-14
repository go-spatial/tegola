//go:build cgo

package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/geom/encoding/wkb"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

const wfsDDL = `CREATE TABLE wfs_sites (
	fid INTEGER PRIMARY KEY,
	geom BLOB,
	name TEXT,
	seats INTEGER
)`

func wfsService(t *testing.T) *features.Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wfs.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(wfsDDL); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	conf := dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{
			{
				"name":               "wfs_sites",
				"tablename":          "wfs_sites",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               4326,
				"fields":             []string{"name", "seats"},
			},
		},
	}
	tiler, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)
	gp := tiler.(*gpkg.Provider)
	layers, err := gp.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	var layer provider.LayerInfo
	for _, l := range layers {
		if l.Name() == "wfs_sites" {
			layer = l
		}
	}
	service, err := features.NewService([]features.CollectionSource{
		{ID: "wfs_sites", Title: "Sites", Layer: layer, Querier: gp},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}

func wfsHandler(t *testing.T, service *features.Service) (*WFSHandler, http.Handler) {
	t.Helper()
	wfsCfg := config.WFSConfig{Enabled: true, BasePath: "/wfs"}.Resolved()
	writeCfg := config.FeaturesWriteConfig{Enabled: true, AuthMode: "dev"}
	writeCfg.Collections = []config.WriteCollectionConfig{
		{ID: "wfs_sites", Operations: []string{"create", "replace", "update", "delete"}},
	}
	h := &WFSHandler{Service: service, Config: wfsCfg, WriteConfig: writeCfg}
	preserveDiscoveryGlobals(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{WFS: h})
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return h, router
}

func TestWFSGetCapabilities(t *testing.T) {
	_, router := wfsHandler(t, wfsService(t))
	for _, v := range []string{"1.1.0", "2.0.0"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetCapabilities&version="+v, nil))
		if rec.Code != 200 {
			t.Fatalf("version %s: status = %d", v, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "WFS_Capabilities") {
			t.Fatalf("version %s: not a capabilities document", v)
		}
		if !strings.Contains(body, "wfs_sites") {
			t.Fatalf("version %s: feature type missing", v)
		}
		if !strings.Contains(body, "Transaction") {
			t.Fatalf("version %s: Transaction not advertised", v)
		}
	}
}

func TestWFSDescribeFeatureType(t *testing.T) {
	_, router := wfsHandler(t, wfsService(t))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=DescribeFeatureType&version=2.0.0&typeNames=wfs_sites", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "xsd:schema") {
		t.Fatal("not an XSD schema")
	}
	if !strings.Contains(body, `name="name"`) || !strings.Contains(body, `name="seats"`) {
		t.Fatal("properties missing from XSD")
	}
}

func TestWFSGetFeature(t *testing.T) {
	service := wfsService(t)
	_, router := wfsHandler(t, service)
	// Seed one feature via the provider directly.
	mp, _, err := service.MutationProviderFor("wfs_sites")
	if err != nil {
		t.Fatalf("mutation provider: %v", err)
	}
	tx, _ := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	out, err := tx.Apply(context.Background(), provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "wfs_sites",
		Properties:  map[string]provider.MutationValue{"name": wfsStrVal("seed")},
		GeometryWKB: wfsWKBPoint(t, 30, 40),
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&typeNames=wfs_sites", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "wfs:FeatureCollection") {
		t.Fatalf("not a feature collection: %s", body[:200])
	}
	// A40: validate XML namespaces (not just substrings).
	{
		var fc struct {
			XMLName xml.Name `xml:"FeatureCollection"`
			Xmlns   string   `xml:"xmlns:wfs,attr"`
			Members []struct {
				XMLName xml.Name `xml:"featureMember"`
				ID      string   `xml:"id,attr"`
			} `xml:"featureMember"`
		}
		// Parse with namespace awareness.
		decoder := xml.NewDecoder(strings.NewReader(body))
		foundWFS := false
		for {
			tok, err := decoder.Token()
			if err != nil {
				break
			}
			if se, ok := tok.(xml.StartElement); ok {
				if se.Name.Local == "FeatureCollection" {
					for _, attr := range se.Attr {
						if attr.Name.Local == "wfs" && attr.Value == "http://www.opengis.net/wfs/2.0" {
							foundWFS = true
						}
					}
				}
			}
		}
		if !foundWFS {
			t.Fatal("WFS 2.0 namespace not declared in GetFeature response")
		}
		_ = fc
	}
	// A31: FID uses NCName-safe encoding (wfs_sites -> wfs_x5F_sites).
	expectedFID, err := feature.EncodeWFSFID("wfs_sites", out.FeatureID)
	if err != nil {
		t.Fatalf("encode FID: %v", err)
	}
	if !strings.Contains(body, expectedFID) {
		t.Fatalf("inserted feature missing from GML (want FID %q)", expectedFID)
	}
	if !strings.Contains(body, "seed") {
		t.Fatal("property value missing from GML")
	}
}

func TestWFSTransaction(t *testing.T) {
	_, router := wfsHandler(t, wfsService(t))
	// Insert via WFS-T.
	insertXML := `<wfs:Transaction version="2.0.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:t="http://example.com/tegola/wfs_sites">
  <wfs:Insert>
    <t:wfs_sites>
      <t:name>wfs-insert</t:name>
      <t:seats>7</t:seats>
      <t:geom><gml:Point srsName="urn:ogc:def:crs:EPSG::4326"><gml:pos>50 60</gml:pos></gml:Point></t:geom>
    </t:wfs_sites>
  </wfs:Insert>
</wfs:Transaction>`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(insertXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("Transaction Insert status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "totalInserted>1") {
		t.Fatalf("insert not acknowledged: %s", body)
	}
	// Extract the assigned FID.
	fid := extractFID(t, body)

	// GetFeature should return it.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=1.1.0&typeName=wfs_sites", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "wfs-insert") {
		t.Fatalf("inserted feature not readable via WFS 1.1: %d", rec.Code)
	}

	// Update via WFS-T.
	updateXML := `<wfs:Transaction version="2.0.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:fes="http://www.opengis.net/fes/2.0">
  <wfs:Update typeName="wfs_sites">
    <wfs:Property><wfs:ValueReference>seats</wfs:ValueReference><wfs:Value>8</wfs:Value></wfs:Property>
    <fes:Filter><fes:ResourceId rid="` + fid + `"/></fes:Filter>
  </wfs:Update>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(updateXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "totalUpdated>1") {
		t.Fatalf("Transaction Update failed: %d %s", rec.Code, rec.Body.String())
	}

	// Delete via WFS-T.
	deleteXML := `<wfs:Transaction version="1.1.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs" xmlns:ogc="http://www.opengis.net/ogc">
  <wfs:Delete typeName="wfs_sites">
    <ogc:Filter><ogc:FeatureId fid="` + fid + `"/></ogc:Filter>
  </wfs:Delete>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(deleteXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "totalDeleted>1") {
		t.Fatalf("Transaction Delete failed: %d %s", rec.Code, rec.Body.String())
	}
}

// WFS 2.0 clients send the plural typeNames attribute on Update/Delete.
// Regression: the parser only read the WFS 1.1 singular typeName, so
// plural-form Deletes failed the R11 fid-belongs-to-type check with
// `feature ID ... does not belong to typeName ""`.
func TestWFSTransactionPluralTypeNames(t *testing.T) {
	_, router := wfsHandler(t, wfsService(t))
	insertXML := `<wfs:Transaction version="2.0.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:t="http://example.com/tegola/wfs_sites">
  <wfs:Insert>
    <t:wfs_sites>
      <t:name>wfs-plural</t:name>
      <t:seats>9</t:seats>
      <t:geom><gml:Point srsName="urn:ogc:def:crs:EPSG::4326"><gml:pos>51 61</gml:pos></gml:Point></t:geom>
    </t:wfs_sites>
  </wfs:Insert>
</wfs:Transaction>`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(insertXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "totalInserted>1") {
		t.Fatalf("Transaction Insert failed: %d %s", rec.Code, rec.Body.String())
	}
	fid := extractFID(t, rec.Body.String())

	// Delete with the WFS 2.0 plural typeNames attribute.
	deleteXML := `<wfs:Transaction version="2.0.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:fes="http://www.opengis.net/fes/2.0">
  <wfs:Delete typeNames="wfs_sites">
    <fes:Filter><fes:ResourceId rid="` + fid + `"/></fes:Filter>
  </wfs:Delete>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(deleteXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "totalDeleted>1") {
		t.Fatalf("Transaction Delete with plural typeNames failed: %d %s", rec.Code, rec.Body.String())
	}

	// Same for Update: plural typeNames must resolve the target layer.
	updateXML := `<wfs:Transaction version="2.0.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:t="http://example.com/tegola/wfs_sites">
  <wfs:Insert>
    <t:wfs_sites>
      <t:name>wfs-plural-upd</t:name>
      <t:seats>10</t:seats>
      <t:geom><gml:Point srsName="urn:ogc:def:crs:EPSG::4326"><gml:pos>52 62</gml:pos></gml:Point></t:geom>
    </t:wfs_sites>
  </wfs:Insert>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(updateXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "totalInserted>1") {
		t.Fatalf("second Transaction Insert failed: %d %s", rec.Code, rec.Body.String())
	}
	fid2 := extractFID(t, rec.Body.String())

	updatePluralXML := `<wfs:Transaction version="2.0.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:fes="http://www.opengis.net/fes/2.0">
  <wfs:Update typeNames="wfs_sites">
    <wfs:Property><wfs:ValueReference>seats</wfs:ValueReference><wfs:Value>11</wfs:Value></wfs:Property>
    <fes:Filter><fes:ResourceId rid="` + fid2 + `"/></fes:Filter>
  </wfs:Update>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(updatePluralXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "totalUpdated>1") {
		t.Fatalf("Transaction Update with plural typeNames failed: %d %s", rec.Code, rec.Body.String())
	}
}

func extractFID(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, `rid="`)
	if i < 0 {
		i = strings.Index(body, `fid="`)
		if i < 0 {
			t.Fatalf("no FID in response: %s", body)
		}
		i += 5
	} else {
		i += 5
	}
	j := strings.Index(body[i:], `"`)
	return body[i : i+j]
}

func wfsStrVal(s string) provider.MutationValue {
	return provider.MutationValue{Kind: provider.MutationValueString, String: s}
}

func wfsWKBPoint(t *testing.T, x, y float64) []byte {
	t.Helper()
	b, err := wkb.EncodeBytes(geom.Point{x, y})
	if err != nil {
		t.Fatalf("wkb: %v", err)
	}
	return b
}

func TestWFSGetFeatureHits(t *testing.T) {
	_, router := wfsHandler(t, wfsService(t))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&typeNames=wfs_sites&resultType=hits", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "numberMatched") {
		t.Fatalf("hits response missing numberMatched: %s", body[:200])
	}
	// Hits must not contain feature members.
	if strings.Contains(body, "wfs:member") {
		t.Fatal("hits response contains feature members")
	}
}

func TestWFSGetFeatureProjection(t *testing.T) {
	service := wfsService(t)
	_, router := wfsHandler(t, service)
	// Seed.
	mp, _, _ := service.MutationProviderFor("wfs_sites")
	tx, _ := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	_, _ = tx.Apply(context.Background(), provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "wfs_sites",
		Properties:  map[string]provider.MutationValue{"name": wfsStrVal("proj"), "seats": wfsIntVal(5)},
		GeometryWKB: wfsWKBPoint(t, 1, 2),
	})
	_, _ = tx.Commit(context.Background())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&typeNames=wfs_sites&propertyName=name", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "proj") {
		t.Fatal("projected property missing")
	}
	if strings.Contains(body, ">5<") || strings.Contains(body, "seats") {
		t.Fatal("non-projected property leaked into response")
	}
}

func TestWFSGetFeatureSortBy(t *testing.T) {
	service := wfsService(t)
	_, router := wfsHandler(t, service)
	mp, _, _ := service.MutationProviderFor("wfs_sites")
	tx, _ := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	for i, name := range []string{"charlie", "alpha", "bravo"} {
		_, _ = tx.Apply(context.Background(), provider.Mutation{
			Op:          provider.MutationInsert,
			Collection:  "wfs_sites",
			Properties:  map[string]provider.MutationValue{"name": wfsStrVal(name), "seats": wfsIntVal(int64(i))},
			GeometryWKB: wfsWKBPoint(t, float64(i), float64(i)),
		})
	}
	_, _ = tx.Commit(context.Background())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&typeNames=wfs_sites&sortBy=name", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	ia := strings.Index(body, "alpha")
	ib := strings.Index(body, "bravo")
	ic := strings.Index(body, "charlie")
	if ia < 0 || ib < 0 || ic < 0 || !(ia < ib && ib < ic) {
		t.Fatalf("sortBy=name order wrong: %d %d %d", ia, ib, ic)
	}

	// DESC.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&typeNames=wfs_sites&sortBy=name+DESC", nil))
	if rec.Code != 200 {
		t.Fatalf("DESC status = %d", rec.Code)
	}
	body = rec.Body.String()
	ia = strings.Index(body, "alpha")
	ic = strings.Index(body, "charlie")
	if !(ic < ia) {
		t.Fatal("sortBy=name DESC order wrong")
	}
}

func wfsIntVal(n int64) provider.MutationValue {
	return provider.MutationValue{Kind: provider.MutationValueInteger, Integer: n}
}

func TestWFSGetPropertyValue(t *testing.T) {
	service := wfsService(t)
	_, router := wfsHandler(t, service)
	mp, _, _ := service.MutationProviderFor("wfs_sites")
	tx, _ := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	_, _ = tx.Apply(context.Background(), provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "wfs_sites",
		Properties:  map[string]provider.MutationValue{"name": wfsStrVal("propval"), "seats": wfsIntVal(9)},
		GeometryWKB: wfsWKBPoint(t, 3, 4),
	})
	_, _ = tx.Commit(context.Background())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetPropertyValue&version=2.0.0&typeNames=wfs_sites&valueReference=name", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ValueCollection") {
		t.Fatalf("not a ValueCollection: %s", body[:200])
	}
	if !strings.Contains(body, "propval") {
		t.Fatal("property value missing")
	}
	// Unknown property -> 400.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetPropertyValue&version=2.0.0&typeNames=wfs_sites&valueReference=nope", nil))
	if rec.Code != 400 {
		t.Fatalf("unknown property status = %d, want 400", rec.Code)
	}
	// WFS 1.1 -> 400 (not supported).
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetPropertyValue&version=1.1.0&typeNames=wfs_sites&valueReference=name", nil))
	if rec.Code != 400 {
		t.Fatalf("WFS 1.1 status = %d, want 400", rec.Code)
	}
}

func TestWFSStoredQueries(t *testing.T) {
	service := wfsService(t)
	_, router := wfsHandler(t, service)
	mp, _, _ := service.MutationProviderFor("wfs_sites")
	tx, _ := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	out, _ := tx.Apply(context.Background(), provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "wfs_sites",
		Properties:  map[string]provider.MutationValue{"name": wfsStrVal("stored")},
		GeometryWKB: wfsWKBPoint(t, 5, 6),
	})
	_, _ = tx.Commit(context.Background())

	// ListStoredQueries.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=ListStoredQueries&version=2.0.0", nil))
	if rec.Code != 200 {
		t.Fatalf("ListStoredQueries status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "GetFeatureById") {
		t.Fatal("mandatory stored query not listed")
	}

	// DescribeStoredQueries.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=DescribeStoredQueries&version=2.0.0&storedQuery_id=urn:ogc:def:query:OGC-WFS::GetFeatureById", nil))
	if rec.Code != 200 {
		t.Fatalf("DescribeStoredQueries status = %d, body: %s", rec.Code, rec.Body.String())
	}

	// GetFeature via stored query.
	fid := "wfs_sites." + strconv.FormatUint(out.FeatureID, 10)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&storedQuery_id=urn:ogc:def:query:OGC-WFS::GetFeatureById&typeName=wfs_sites&id="+fid, nil))
	if rec.Code != 200 {
		t.Fatalf("stored query GetFeature status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "stored") {
		t.Fatal("stored query did not return the feature")
	}

	// Unknown stored query -> 400.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=GetFeature&version=2.0.0&storedQuery_id=urn:unknown", nil))
	if rec.Code != 400 {
		t.Fatalf("unknown stored query status = %d, want 400", rec.Code)
	}
}

func TestWFSLockFeature(t *testing.T) {
	service := wfsService(t)
	h, router := wfsHandler(t, service)
	// Enable write for lock test.
	h.WriteConfig.Enabled = true
	mp, _, _ := service.MutationProviderFor("wfs_sites")
	tx, _ := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	out, _ := tx.Apply(context.Background(), provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "wfs_sites",
		Properties:  map[string]provider.MutationValue{"name": wfsStrVal("locked")},
		GeometryWKB: wfsWKBPoint(t, 7, 8),
	})
	_, _ = tx.Commit(context.Background())
	fid := "wfs_sites." + strconv.FormatUint(out.FeatureID, 10)

	// Lock the feature.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wfs?service=WFS&request=LockFeature&version=1.1.0&typeName=wfs_sites&featureId="+fid, nil))
	if rec.Code != 200 {
		t.Fatalf("LockFeature status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "LockId") {
		t.Fatalf("no LockId in response: %s", body)
	}
	lockID := extractLockID(t, body)

	// Transaction without lockId on locked feature -> 403.
	updateXML := `<wfs:Transaction version="1.1.0" service="WFS" xmlns:wfs="http://www.opengis.net/wfs" xmlns:ogc="http://www.opengis.net/ogc">
  <wfs:Update typeName="wfs_sites">
    <wfs:Property><wfs:Name>name</wfs:Name><wfs:Value>hacked</wfs:Value></wfs:Property>
    <ogc:Filter><ogc:FeatureId fid="` + fid + `"/></ogc:Filter>
  </wfs:Update>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(updateXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("locked update without lockId status = %d, want 403", rec.Code)
	}

	// Transaction with lockId -> 200.
	updateXML = `<wfs:Transaction version="1.1.0" service="WFS" lockId="` + lockID + `" xmlns:wfs="http://www.opengis.net/wfs" xmlns:ogc="http://www.opengis.net/ogc">
  <wfs:Update typeName="wfs_sites">
    <wfs:Property><wfs:Name>name</wfs:Name><wfs:Value>unlocked</wfs:Value></wfs:Property>
    <ogc:Filter><ogc:FeatureId fid="` + fid + `"/></ogc:Filter>
  </wfs:Update>
</wfs:Transaction>`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(updateXML))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("locked update with lockId status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func extractLockID(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, "<wfs:LockId>")
	if i < 0 {
		t.Fatalf("no LockId: %s", body)
	}
	i += len("<wfs:LockId>")
	j := strings.Index(body[i:], "</wfs:LockId>")
	return body[i : i+j]
}
