package wfs

import "testing"

func TestOutputFormatRejectsUnsupportedBeforeQueryDispatch(t *testing.T) {
	for _, version := range []Version{V110, V200, V202} {
		for _, format := range []string{"application/json", "GML2", "text/csv", "application/gml+xml; version=2.1", "application/gml+xml; unexpected=yes"} {
			for _, stored := range []bool{false, true} {
				q := map[string]string{"typenames": "roads", "outputformat": format}
				if stored {
					q["storedquery_id"] = "urn:ogc:def:query:OGC-WFS::GetFeatureById"
					q["id"] = "roads.1"
				}
				_, exceptions := ParseGetFeatureKVP(version, q)
				if len(exceptions) == 0 || exceptions[0].Locator != "outputFormat" {
					t.Errorf("version=%s format=%q stored=%v: %v", version, format, stored, exceptions)
				}
			}
		}
	}
}

func TestOutputFormatAcceptsVersionedGML(t *testing.T) {
	for _, tc := range []struct {
		version Version
		format  string
	}{
		{V110, ""}, {V200, "application/gml+xml"}, {V202, "application/xml"},
		{V110, "text/xml; subtype=gml/3.1.1"},
		{V110, "application/gml+xml; version=3.1.1"},
		{V200, "application/gml+xml; version=3.2"},
		{V202, "application/gml+xml; version=3.2; charset=UTF-8"},
	} {
		if _, exceptions := ParseGetFeatureKVP(tc.version, map[string]string{"typenames": "roads", "outputformat": tc.format}); len(exceptions) != 0 {
			t.Errorf("%s %q: %v", tc.version, tc.format, exceptions)
		}
	}
	for _, version := range []Version{V200, V202} {
		_, exceptions := ParseGetPropertyValueKVP(version, map[string]string{"typenames": "roads", "valuereference": "title", "outputformat": "application/json"})
		if len(exceptions) == 0 || exceptions[0].Locator != "outputFormat" {
			t.Errorf("GPV %s accepted JSON: %v", version, exceptions)
		}
	}
}
