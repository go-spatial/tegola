package wfs

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/provider"
)

func TestTransactionPreservesOrderAndText(t *testing.T) {
	actions, _, _, err := ParseTransaction(V200, []byte(`<Transaction><Insert handle="batch"><sites><name> first </name></sites><sites><name></name></sites><sites><name>third</name></sites></Insert></Transaction>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 3 {
		t.Fatalf("got %d actions", len(actions))
	}
	for i, want := range []string{" first ", "", "third"} {
		got, ok := actions[i].Properties["name"]
		if !ok || got != want || actions[i].Handle != "batch" {
			t.Errorf("action %d = %#v", i, actions[i])
		}
	}
}

func TestRequestVersionPreserved(t *testing.T) {
	v, _, err := ParseVersionedRequest([]byte(`<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0" version="2.0.0"/>`))
	if err != nil || v != V200 {
		t.Fatalf("got %s %v", v, err)
	}
	v, err = Negotiate("", []string{"1.1.0", "2.0.2"})
	if err != nil || v != V202 {
		t.Fatalf("got %s %v", v, err)
	}
}

func TestTransactionGeometryCRSAndReplace(t *testing.T) {
	for _, op := range []string{"Insert", "Replace"} {
		filter := ""
		if op == "Replace" {
			filter = `<Filter><ResourceId rid="sites.2"/></Filter>`
		}
		body := `<Transaction><` + op + `><sites><name>valid</name><geometry><Point srsName="EPSG:3857"><pos>1000 2000</pos></Point></geometry></sites>` + filter + `</` + op + `></Transaction>`
		actions, _, _, err := ParseTransaction(V200, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		schema := &feature.SchemaDescriptor{Properties: []feature.PropertyDescriptor{{Name: "name", Type: feature.TypeString}}}
		ms, err := actionToMutation(V200, schema, actions[0])
		if err != nil {
			t.Fatal(err)
		}
		if ms[0].GeometrySRID != 3857 {
			t.Fatalf("%s SRID=%d", op, ms[0].GeometrySRID)
		}
		if len(ms[0].Properties) != 1 {
			t.Fatalf("Filter leaked into properties: %#v", ms[0].Properties)
		}
	}
}

func TestTransactionResponseGroupsInsertResults(t *testing.T) {
	out := TransactionResponse(V200, []TransactionResult{{Op: provider.MutationInsert, TypeName: "sites", FeatureID: 1, Affected: 1}, {Op: provider.MutationInsert, TypeName: "sites", FeatureID: 2, Affected: 1}})
	var doc struct {
		Results []struct {
			Features []struct{} `xml:"Feature"`
		} `xml:"InsertResults"`
	}
	if err := xml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Results) != 1 || len(doc.Results[0].Features) != 2 {
		t.Fatalf("invalid insert result grouping: %s", out)
	}
	if strings.Index(out, "totalReplaced") > strings.Index(out, "totalDeleted") {
		t.Fatalf("incorrect summary sequence: %s", out)
	}
}

func TestTransactionRejectsDimensionalGeometry(t *testing.T) {
	actions, _, _, err := ParseTransaction(V200, []byte(`<Transaction><Insert><sites><geometry><LineString><posList srsDimension="3">1 2 3 4 5 6</posList></LineString></geometry></sites></Insert></Transaction>`))
	if err != nil {
		return
	}
	_, err = insertActionToMutation(V200, &feature.SchemaDescriptor{}, actions[0])
	if err == nil {
		t.Fatal("accepted XYZ after dropping nested srsDimension")
	}
}

func TestTransactionRejectsUnsupportedActionsAndMissingValue(t *testing.T) {
	for _, raw := range []string{
		`<Transaction><Insert><sites><name>a</name></sites></Insert></Transaction><Transaction/>`,
		`<Transaction><Insert><sites><name>a</name></sites></Insert><Unsupported/></Transaction>`,
		`<Transaction><Delete typeName="sites"><Filter><ResourceId rid="sites.1"/></Filter><Filter><ResourceId rid="sites.2"/></Filter></Delete></Transaction>`,
		`<Transaction><Update typeName="sites"><Property><ValueReference>name</ValueReference></Property><Filter><ResourceId rid="sites.1"/></Filter></Update></Transaction>`,
		`<Transaction releaseAction="SOME"><Delete typeName="sites"><Filter><ResourceId rid="sites.1"/></Filter></Delete></Transaction>`,
	} {
		if _, _, _, err := ParseTransaction(V200, []byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
