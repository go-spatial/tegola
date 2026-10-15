package wfs

import (
	"fmt"
	"github.com/alexeydott/tegola/feature"
	"strings"
	"testing"
)

func TestExplicitScalarNullReview(t *testing.T) {
	for _, version := range []Version{V110, V200, V202} {
		for _, action := range []string{`<Insert><sites><note xsi:nil="true"/></sites></Insert>`, `<Update typeName="sites"><Property><ValueReference>note</ValueReference><Value xsi:nil="1"/></Property><Filter><FeatureId fid="sites.1"/></Filter></Update>`} {
			body := fmt.Sprintf(`<Transaction version="%s" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">%s</Transaction>`, version, action)
			acts, _, _, err := ParseTransaction(version, []byte(body))
			if err != nil {
				t.Fatalf("%s: %v", version, err)
			}
			schema := &feature.SchemaDescriptor{Properties: []feature.PropertyDescriptor{{Name: "note", Type: feature.TypeString, Nullable: true}}}
			ms, err := actionToMutation(version, schema, acts[0])
			if err != nil || len(ms) != 1 || !ms[0].Properties["note"].Null {
				t.Fatalf("null conversion: %+v %v", ms, err)
			}
		}
	}
}

func TestExplicitScalarNullInvalidReview(t *testing.T) {
	for _, field := range []string{
		`<note nil="true"/>`, `<note xmlns:bad="urn:bad" bad:nil="true"/>`,
		`<note xsi:nil="maybe"/>`, `<note xsi:nil="true"> </note>`,
		`<note xsi:nil="true"><child/></note>`, `<Point xsi:nil="true"/>`,
	} {
		body := `<Transaction xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><Insert><sites>` + field + `</sites></Insert></Transaction>`
		if _, _, _, err := ParseTransaction(V200, []byte(body)); err == nil {
			t.Errorf("accepted %s", field)
		}
	}
	for _, value := range []string{`<Value xsi:nil="true">x</Value>`, `<Value xsi:nil="true"><Point/></Value>`, `<Value nil="true"/>`, `<Value xsi:nil="TRUE"/>`} {
		body := `<Transaction xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><Update typeName="sites"><Property><ValueReference>note</ValueReference>` + value + `</Property><Filter><FeatureId fid="sites.1"/></Filter></Update></Transaction>`
		if _, _, _, err := ParseTransaction(V200, []byte(body)); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
}

func TestExplicitScalarNullSchemaAndEmptyReview(t *testing.T) {
	for _, op := range []string{"Insert", "Update", "Replace"} {
		field := `<note xsi:nil="true"/>`
		action := `<` + op + `><sites>` + field + `</sites>`
		if op == "Update" {
			action = `<Update typeName="sites"><Property><ValueReference>note</ValueReference><Value xsi:nil="true"/></Property>`
		}
		if op != "Insert" {
			action += `<Filter><FeatureId fid="sites.1"/></Filter>`
		}
		action += `</` + op + `>`
		acts, _, _, err := ParseTransaction(V200, []byte(`<Transaction xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`+action+`</Transaction>`))
		if err != nil {
			t.Fatal(err)
		}
		schema := &feature.SchemaDescriptor{Properties: []feature.PropertyDescriptor{{Name: "note", Type: feature.TypeString}}}
		if _, err = actionToMutation(V200, schema, acts[0]); err == nil {
			t.Fatalf("%s accepted nonnullable", op)
		}
		schema.Properties[0].Nullable = true
		ms, err := actionToMutation(V200, schema, acts[0])
		if err != nil || !ms[0].Properties["note"].Null {
			t.Fatalf("%s null lost: %v", op, err)
		}
		schema.Geometry.Name = "note"
		schema.Properties = nil
		if _, err = actionToMutation(V200, schema, acts[0]); err == nil {
			t.Fatalf("%s accepted nil geometry", op)
		}
	}
	for _, value := range []string{`<note/>`, `<note xsi:nil="false"/>`, `<note xsi:nil="0"/>`} {
		acts, _, _, err := ParseTransaction(V200, []byte(`<Transaction xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><Insert><sites>`+value+`</sites></Insert></Transaction>`))
		if err != nil {
			t.Fatal(err)
		}
		schema := &feature.SchemaDescriptor{Properties: []feature.PropertyDescriptor{{Name: "note", Type: feature.TypeString, Nullable: true}}}
		ms, err := actionToMutation(V200, schema, acts[0])
		if err != nil || ms[0].Properties["note"].Null || !ms[0].Properties["note"].Empty {
			t.Fatalf("empty changed: %v %+v", err, ms)
		}
	}
}

func TestBrowserInheritedNullNamespaceReview(t *testing.T) {
	raw := `<Transaction xmlns="http://www.opengis.net/wfs/2.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" service="WFS" version="2.0.0"><Update xmlns:feature="http://example.com/tegola/mysql_drainage_pumpstations" typeName="feature:mysql_drainage_pumpstations"><Property><ValueReference>SQL_FIELD_ALIAS_0</ValueReference><Value xsi:nil="true"/></Property><fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0"><fes:ResourceId rid="mysql_x5F_drainage_x5F_pumpstations.26"/></fes:Filter></Update></Transaction>`
	raw = strings.Replace(raw, `service="WFS"`, `xsi:schemaLocation="http://www.opengis.net/wfs/2.0 http://schemas.opengis.net/wfs/2.0/wfs.xsd" service="WFS"`, 1)
	acts, _, _, err := ParseTransaction(V200, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || !acts[0].NullProperties["SQL_FIELD_ALIAS_0"] {
		t.Fatalf("null lost: %+v", acts)
	}
	for _, spoof := range []string{
		strings.ReplaceAll(raw, "http://www.w3.org/2001/XMLSchema-instance", "urn:foreign"),
		strings.Replace(raw, `<Value xsi:nil=`, `<Value xmlns:xsi="urn:foreign" xsi:nil=`, 1),
	} {
		if _, _, _, err := ParseTransaction(V200, []byte(spoof)); err == nil {
			t.Fatal("accepted foreign nil namespace")
		}
	}
	plain := strings.Replace(raw, `xsi:schemaLocation="http://www.opengis.net/wfs/2.0 http://schemas.opengis.net/wfs/2.0/wfs.xsd" `, "", 1)
	if _, _, _, err := ParseTransaction(V200, []byte(plain)); err != nil {
		t.Fatal(err)
	}
}
