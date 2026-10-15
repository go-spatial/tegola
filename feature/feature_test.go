package feature

import (
	"testing"
)

func testSchema() *SchemaDescriptor {
	return &SchemaDescriptor{
		Collection: "parcels",
		IDColumn:   "fid",
		Properties: []PropertyDescriptor{
			{Name: "name", Column: "name", Type: TypeString, Nullable: true, MaxLength: 100},
			{Name: "area", Column: "area_m2", Type: TypeDecimal, Nullable: true},
			{Name: "lots", Column: "lots", Type: TypeInteger, Nullable: false, Required: true},
			{Name: "code", Column: "code", Type: TypeString, ReadOnly: true},
		},
		Geometry: GeometryDescriptor{Name: "geometry", Column: "geom", Type: "polygon", Dimension: DimXY, SRID: 4326},
	}
}

func TestValidateInputValue(t *testing.T) {
	s := testSchema()
	cases := []struct {
		name    string
		prop    string
		val     TypedValue
		wantErr bool
	}{
		{"ok string", "name", TypedValue{Type: TypeString, State: ValuePresent, String: "a"}, false},
		{"unknown property", "nope", TypedValue{Type: TypeString, State: ValuePresent}, true},
		{"read-only rejected", "code", TypedValue{Type: TypeString, State: ValuePresent, String: "x"}, true},
		{"type mismatch", "lots", TypedValue{Type: TypeString, State: ValuePresent, String: "x"}, true},
		{"null allowed", "name", TypedValue{Type: TypeString, State: ValueNull}, false},
		{"null on non-nullable", "lots", TypedValue{Type: TypeInteger, State: ValueNull}, true},
		{"absent always ok", "lots", TypedValue{State: ValueAbsent}, false},
		{"too long", "name", TypedValue{Type: TypeString, State: ValuePresent, String: string(make([]byte, 101))}, true},
		{"bad decimal", "area", TypedValue{Type: TypeDecimal, State: ValuePresent, Decimal: "12.3.4"}, true},
		{"good decimal", "area", TypedValue{Type: TypeDecimal, State: ValuePresent, Decimal: "-12.30"}, false},
		{"integer ok", "lots", TypedValue{Type: TypeInteger, State: ValuePresent, Integer: 7}, false},
	}
	for _, c := range cases {
		err := s.ValidateInputValue(c.prop, c.val)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: got err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestValidateRequired(t *testing.T) {
	s := testSchema()
	if err := s.ValidateRequired(map[string]TypedValue{
		"lots": {Type: TypeInteger, State: ValuePresent, Integer: 1},
	}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := s.ValidateRequired(map[string]TypedValue{}); err == nil {
		t.Fatal("expected missing required error")
	}
}

func TestFIDRoundTrip(t *testing.T) {
	fid, err := EncodeWFSFID("parcels", 42)
	if err != nil {
		t.Fatal(err)
	}
	if fid != "parcels.42" {
		t.Fatalf("got %q", fid)
	}
	coll, id, err := DecodeWFSFID(fid)
	if err != nil {
		t.Fatal(err)
	}
	if coll != "parcels" || id != 42 {
		t.Fatalf("got %q %d", coll, id)
	}
	if _, _, err := DecodeWFSFID("no-dot"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := DecodeWFSFID(".42"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodeWFSFIDSanitizes(t *testing.T) {
	fid, err := EncodeWFSFID("my collection!", 1)
	if err != nil {
		t.Fatal(err)
	}
	// Must be NCName-safe: no spaces or bangs.
	for _, r := range fid {
		if r == ' ' || r == '!' {
			t.Fatalf("unsafe char in %q", fid)
		}
	}
}

// A31: FID encoding must be injective and NCName-valid.
// 'a:b' and 'a_b' must produce distinct FIDs; '%' is illegal in NCName.
func TestFIDInjectiveNCName(t *testing.T) {
	fid1, err := EncodeWFSFID("a:b", 1)
	if err != nil {
		t.Fatal(err)
	}
	fid2, err := EncodeWFSFID("a_b", 1)
	if err != nil {
		t.Fatal(err)
	}
	if fid1 == fid2 {
		t.Fatalf("not injective: %q == %q", fid1, fid2)
	}
	// No '%' in output (illegal in XML NCName).
	for _, fid := range []string{fid1, fid2} {
		for _, r := range fid {
			if r == '%' {
				t.Fatalf("illegal percent in NCName %q", fid)
			}
		}
	}
	// Round-trip.
	for _, tc := range []struct {
		coll string
		id   uint64
	}{
		{"a:b", 1}, {"a_b", 2}, {"caf\u00e9", 3}, {"_lead", 4}, {"9start", 5},
	} {
		fid, err := EncodeWFSFID(tc.coll, tc.id)
		if err != nil {
			t.Fatal(err)
		}
		gotColl, gotID, err := DecodeWFSFID(fid)
		if err != nil {
			t.Fatalf("decode %q: %v", fid, err)
		}
		if gotColl != tc.coll || gotID != tc.id {
			t.Fatalf("round-trip %q: got %q,%d want %q,%d", fid, gotColl, gotID, tc.coll, tc.id)
		}
	}
}

func TestPhysicalFeatureKey(t *testing.T) {
	k := PhysicalFeatureKey{Domain: "gpkg", Relation: "parcels", PK: "42"}
	if k.String() != "gpkg.parcels.42" {
		t.Fatalf("got %q", k.String())
	}
}

func TestTypedValueConversion(t *testing.T) {
	if v := toTypedValue(nil); v.State != ValueNull {
		t.Fatal("nil should be null")
	}
	if v := toTypedValue(int64(5)); v.Type != TypeInteger || v.Integer != 5 {
		t.Fatal("int64")
	}
	if v := toTypedValue(""); v.State != ValueEmpty {
		t.Fatal("empty string")
	}
	if v := toTypedValue("x"); v.State != ValuePresent || v.String != "x" {
		t.Fatal("string")
	}
	if v := toTypedValue(true); v.Type != TypeBoolean || !v.Boolean {
		t.Fatal("bool")
	}
}
