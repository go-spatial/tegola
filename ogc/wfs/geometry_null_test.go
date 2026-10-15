package wfs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/feature"
)

func TestTransactionGeometryNullStates(t *testing.T) {
	for _, version := range []Version{V110, V200, V202} {
		for _, op := range []string{"Insert", "Update", "Replace"} {
			if version == V110 && op == "Replace" {
				continue // Replace is not part of WFS 1.1.
			}
			for _, value := range []string{`<geom xsi:nil="true"/>`, `<geom xsi:nil="1"/>`, `<geom/>`, `<geom xsi:nil="false"/>`, ""} {
				t.Run(fmt.Sprintf("%s/%s/%s", version, op, value), func(t *testing.T) {
					action := "<" + op + "><sites>" + value + "</sites>"
					if op == "Update" {
						property := ""
						if value != "" {
							// Reuse the same lexical nil state on the protocol Value element.
							property = "<Property><ValueReference>geom</ValueReference>" + strings.ReplaceAll(value, "geom", "Value") + "</Property>"
						}
						action = `<Update typeName="sites">` + property
					}
					if op != "Insert" {
						action += `<Filter><FeatureId fid="sites.1"/></Filter>`
					}
					action += "</" + op + ">"
					body := fmt.Sprintf(`<Transaction version="%s" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">%s</Transaction>`, version, action)
					acts, _, _, err := ParseTransaction(version, []byte(body))
					if err != nil {
						t.Fatal(err)
					}
					isNull := acts[0].NullProperties["geom"]
					for _, nullable := range []bool{false, true} {
						schema := &feature.SchemaDescriptor{Geometry: feature.GeometryDescriptor{Name: "geom", Nullable: nullable}}
						mutations, err := actionToMutation(version, schema, acts[0])
						wantError := (isNull && !nullable) || (value != "" && !isNull)
						if wantError {
							if err == nil {
								t.Fatalf("nullable=%v accepted invalid geometry", nullable)
							}
							continue
						}
						if err != nil || len(mutations) != 1 {
							t.Fatalf("nullable=%v: %+v %v", nullable, mutations, err)
						}
						m := mutations[0]
						if m.GeometryAbsent != isNull || m.GeometryWKB != nil || len(m.Properties) != 0 {
							t.Fatalf("geometry state lost: %+v", m)
						}
					}
				})
			}
		}
	}
}
