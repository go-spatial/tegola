package mos

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
)

// systemInfoBlobBuilder assembles a MapplGIS LayerInfo blob mirroring the
// packed MapplGIS record layout.
type systemInfoBlobBuilder struct {
	version         string
	precision       int32
	hasProjection   bool
	projection      string
	layerID         int32
	gridWidth       float64
	gridHeight      float64
	mapUnits        byte
	unitsDefined    bool
	projectionSize  int32 // override; -1 means derive from projection
	projectionStart int   // override of the 64-byte fixed part length
}

func (b systemInfoBlobBuilder) build() []byte {
	buf := make([]byte, 64+len(b.projection))
	// Version string[10]: length prefix + text + NUL padding
	buf[0] = byte(len(b.version))
	copy(buf[1:], b.version)
	binary.LittleEndian.PutUint32(buf[11:15], uint32(b.precision))
	if b.hasProjection {
		buf[15] = 1
	}
	binary.LittleEndian.PutUint32(buf[26:30], uint32(b.layerID))
	binary.LittleEndian.PutUint64(buf[36:44], math.Float64bits(b.gridWidth))
	binary.LittleEndian.PutUint64(buf[44:52], math.Float64bits(b.gridHeight))
	buf[52] = b.mapUnits
	if b.unitsDefined {
		buf[53] = 1
	}
	projStart := b.projectionStart
	if projStart == 0 {
		projStart = 64
	}
	size := b.projectionSize
	if size == 0 {
		size = int32(len(b.projection))
	}
	binary.LittleEndian.PutUint32(buf[60:64], uint32(size))
	copy(buf[projStart:], b.projection)
	return buf
}

func TestParseSystemInfo(t *testing.T) {
	buf := systemInfoBlobBuilder{
		version:       "Ver 1",
		precision:     2,
		hasProjection: true,
		projection:    "+proj=merc +ellps=WGS84 +datum=WGS84 +units=m +no_defs",
		layerID:       42,
		gridWidth:     1000,
		gridHeight:    1000,
		mapUnits:      byte(UnitsMillimetres),
		unitsDefined:  true,
	}.build()

	si, err := ParseSystemInfo(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Precision != 2 {
		t.Errorf("Precision = %v, want 2", si.Precision)
	}
	if si.LayerID != 42 {
		t.Errorf("LayerID = %v, want 42", si.LayerID)
	}
	if si.Projection != "+proj=merc +ellps=WGS84 +datum=WGS84 +units=m +no_defs" {
		t.Errorf("Projection = %q", si.Projection)
	}
	if si.MapUnits != UnitsMillimetres || !si.MapUnitsDefined {
		t.Errorf("MapUnits = %v defined=%v, want millimetres defined", si.MapUnits, si.MapUnitsDefined)
	}

	// the observed real-world case: precision 2, units defined as false
	// (coordinates already in CRS units), no map units scaling
	buf = systemInfoBlobBuilder{
		version:       "Ver 1",
		precision:     2,
		hasProjection: true,
		projection:    "+proj=merc +ellps=WGS84 +datum=WGS84 +units=m +no_defs",
		mapUnits:      byte(UnitsMillimetres),
		unitsDefined:  false,
	}.build()
	si, err = ParseSystemInfo(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.MapUnitsDefined {
		t.Error("MapUnitsDefined = true, want false")
	}
	if scale, err := si.ScaleToMetres(); err != nil || scale != 1 {
		t.Errorf("ScaleToMetres() = %v, %v; want 1, nil", scale, err)
	}
}

func TestParseSystemInfoInvalid(t *testing.T) {
	// a MOS geometry blob must not be mistaken for a system info record
	mosBlob := (&blobBuilder{oType: TypePolyline, counts: []uint32{2}, points: [][2]int32{{1, 2}, {3, 4}}}).build()
	if IsSystemInfoBlob(mosBlob) {
		t.Error("IsSystemInfoBlob(mosBlob) = true, want false")
	}
	if _, err := ParseSystemInfo(mosBlob); err == nil {
		t.Error("ParseSystemInfo(mosBlob) succeeded, want error")
	}

	// short buffer
	if IsSystemInfoBlob([]byte{5, 'V', 'e', 'r', ' ', '1'}) {
		t.Error("IsSystemInfoBlob(short) = true, want false")
	}

	// version marker mismatch
	buf := systemInfoBlobBuilder{version: "Xer 1", precision: 1}.build()
	if IsSystemInfoBlob(buf) {
		t.Error("IsSystemInfoBlob(wrong version) = true, want false")
	}

	// projection size beyond the blob
	buf = systemInfoBlobBuilder{version: "Ver 1", hasProjection: true, projectionSize: 999}.build()
	if _, err := ParseSystemInfo(buf); err == nil {
		t.Error("ParseSystemInfo(oversized projection) succeeded, want error")
	}
}

func TestUnitsToMetres(t *testing.T) {
	cases := []struct {
		units MapUnits
		want  float64
		err   bool
	}{
		{UnitsMillimetres, 0.001, false},
		{UnitsCentimetres, 0.01, false},
		{UnitsDecimetres, 0.1, false},
		{UnitsMetres, 1, false},
		{UnitsKilometres, 1000, false},
		{MapUnits(5), 0, true}, // muGrad - not supported
		{MapUnits(6), 0, true}, // muNone - not supported
	}
	for _, c := range cases {
		got, err := c.units.ToMetres()
		if c.err {
			if err == nil {
				t.Errorf("units %v: expected error", c.units)
			}
			continue
		}
		if err != nil {
			t.Errorf("units %v: unexpected error %v", c.units, err)
			continue
		}
		if got != c.want {
			t.Errorf("units %v: got %v, want %v", c.units, got, c.want)
		}
	}
}

func TestParseMapUnits(t *testing.T) {
	cases := []struct {
		value string
		want  MapUnits
	}{
		{"mm", UnitsMillimetres},
		{"muMm", UnitsMillimetres},
		{"centimeters", UnitsCentimetres},
		{"muSm", UnitsCentimetres},
		{"dm", UnitsDecimetres},
		{"muDm", UnitsDecimetres},
		{"m", UnitsMetres},
		{"muM", UnitsMetres},
		{"km", UnitsKilometres},
		{"muKm", UnitsKilometres},
	}
	for _, c := range cases {
		got, err := ParseMapUnits(c.value)
		if err != nil {
			t.Errorf("ParseMapUnits(%q): unexpected error: %v", c.value, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMapUnits(%q) = %v, want %v", c.value, got, c.want)
		}
	}
	if _, err := ParseMapUnits("degrees"); err == nil {
		t.Error(`ParseMapUnits("degrees") succeeded, want error`)
	}
}

func TestScaleToMetres(t *testing.T) {
	cases := []struct {
		units    byte
		unitsDef bool
		want     float64
		wantErr  bool
	}{
		{byte(UnitsMetres), true, 1, false},
		{byte(UnitsMillimetres), true, 0.001, false},
		{byte(UnitsKilometres), true, 1000, false},
		{byte(UnitsMetres), false, 1, false},
		{5, true, 0, true}, // muGrad defined - unsupported
		{5, false, 1, false},
	}
	for _, c := range cases {
		si := SystemInfo{MapUnits: MapUnits(c.units), MapUnitsDefined: c.unitsDef}
		got, err := si.ScaleToMetres()
		if c.wantErr {
			if err == nil {
				t.Errorf("units %v defined=%v: expected error", c.units, c.unitsDef)
			}
			continue
		}
		if err != nil {
			t.Errorf("units %v defined=%v: unexpected error %v", c.units, c.unitsDef, err)
			continue
		}
		if got != c.want {
			t.Errorf("units %v defined=%v: got %v, want %v", c.units, c.unitsDef, got, c.want)
		}
	}
}

func TestDecodeUnitFactor(t *testing.T) {
	// a line stored with millimetre-unit coordinates (precision 2 means
	// hundredths of a millimetre); decode with UnitFactor 0.001 to metres
	pts := [][2]int32{{100000, 200000}, {300000, 400000}}
	buf := (&blobBuilder{oType: TypePolyline, counts: []uint32{uint32(len(pts))}, points: pts}).build()

	g, err := Decode(buf, Options{Precision: 2, UnitFactor: 0.001})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ls, ok := g.(geom.LineString)
	if !ok {
		t.Fatalf("expected a geom.LineString, got %T", g)
	}
	want := geom.LineString{{1, 2}, {3, 4}}
	if !reflect.DeepEqual(ls, want) {
		t.Errorf("linestring = %v, want %v", ls, want)
	}
}
