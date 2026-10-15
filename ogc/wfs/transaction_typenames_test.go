package wfs

import (
	"fmt"
	"strings"
	"testing"
)

func TestTransactionPluralTypeNames(t *testing.T) {
	for _, version := range []Version{V200, V202} {
		for _, operation := range []string{"Update", "Delete"} {
			for _, name := range []string{"sites", "app:sites"} {
				t.Run(string(version)+"/"+operation+"/"+name, func(t *testing.T) {
					body := fmt.Sprintf(
						`<wfs:Transaction version="%s"
 xmlns:wfs="http://www.opengis.net/wfs/2.0"
 xmlns:app="http://example.com/tegola/sites" xmlns:fes="http://www.opengis.net/fes/2.0">
 <wfs:%s typeNames="%s"><fes:Filter><fes:ResourceId rid="app:sites.1"/></fes:Filter></wfs:%s>
</wfs:Transaction>`,
						version,
						operation,
						name,
						operation,
					)
					actions, _, _, err := ParseTransaction(version, []byte(body))
					if err != nil {
						t.Fatal(err)
					}
					if len(actions) != 1 {
						t.Fatalf("unexpected transaction actions: %#v", actions)
					}
					if actions[0].TypeName != "sites" || len(actions[0].FilterIDs) != 1 {
						t.Fatalf("unexpected transaction actions: %#v", actions)
					}
					if actions[0].FilterIDs[0] != 1 {
						t.Fatalf("unexpected transaction actions: %#v", actions)
					}
				})
			}
		}
	}
}

func TestTransactionPluralTypeNamesRejectInvalidBindings(t *testing.T) {
	valid := `<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0"
 xmlns:app="http://example.com/tegola/sites" xmlns:fes="http://www.opengis.net/fes/2.0">
 <wfs:Delete typeNames="app:sites"><fes:Filter><fes:ResourceId rid="app:sites.1"/></fes:Filter></wfs:Delete>
</wfs:Transaction>`
	for name, body := range map[string]string{
		"foreign namespace":   strings.ReplaceAll(valid, "http://example.com/tegola/sites", "http://foreign.example/sites"),
		"unknown prefix":      strings.Replace(valid, `typeNames="app:sites"`, `typeNames="missing:sites"`, 1),
		"multiple types":      strings.Replace(valid, `typeNames="app:sites"`, `typeNames="sites roads"`, 1),
		"conflicting aliases": strings.Replace(valid, `typeNames="app:sites"`, `typeName="roads" typeNames="app:sites"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, err := ParseTransaction(V200, []byte(body)); err == nil {
				t.Fatal("accepted invalid transaction type names")
			}
		})
	}
}
