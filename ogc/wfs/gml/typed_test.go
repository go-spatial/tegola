package gml

import (
	"strings"
	"testing"
)

// A30: typed property encoding preserves types.
func TestFormatGMLValueTyped(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{"hello", "hello"},
		{"a<b", "a<b"}, // XML escaping is done by caller
		{int64(42), "42"},
		{int(7), "7"},
		{float64(3.14), "3.14"},
		{float64(1.0), "1"}, // %g: no trailing .0
		{float64(1000000), "1e+06"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
	}
	for _, tc := range cases {
		got := formatGMLValue(tc.in)
		if got != tc.want {
			t.Errorf("formatGMLValue(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A30: GML feature with typed properties encodes without fmt.Sprintf loss.
func TestGMLTypedProperties(t *testing.T) {
	f := Feature{
		ID:           "test.1",
		TypeName:     "test",
		GeometryName: "geometry",
		Properties: map[string]interface{}{
			"name":   "foo",
			"count":  int64(42),
			"ratio":  float64(3.14),
			"active": true,
		},
	}
	enc := &Encoder{Version: V321, SRID: 4326}
	if err := enc.EncodeFeature(f); err != nil {
		t.Fatal(err)
	}
	out := enc.String()
	// Check typed values appear correctly
	for _, want := range []string{"<name>foo</name>", "<count>42</count>", "<ratio>3.14</ratio>", "<active>true</active>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Ensure no Go-syntax artifacts
	if strings.Contains(out, "<nil>") {
		t.Errorf("nil rendered as <nil>")
	}
}
