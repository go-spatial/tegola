// This file implements parsing of the MapplGIS LayerInfo version wrapper
// blob that MapplGIS stores as the first row of every MOS geometry table
// (MapplGIS packed record layout). The blob carries the layer's self-description:
// quantization precision, map units and (optionally) the layer PROJ.4
// definition, letting the provider auto-configure instead of hard-coding
// srid/mos_precision in the config.
package mos

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// MapUnits enumerates the linear TMapUnits values supported for MOS
// coordinate dequantization (MapplGIS types TMapUnits). Angular and
// undefined units (muGrad, muNone) are not supported.
type MapUnits byte

const (
	UnitsMillimetres MapUnits = 0 // muMm
	UnitsCentimetres MapUnits = 1 // muSm
	UnitsDecimetres  MapUnits = 2 // muDm
	UnitsMetres      MapUnits = 3 // muM
	UnitsKilometres  MapUnits = 4 // muKm
)

// unitsToMetres mirrors MapplGIS's ConvertUnitCoeff[units] /
// 10^ConvertUnitExponent[muM] (MapplGIS unit-conversion helpers, used by
// TMapObject.ConvertMosFromProjected): the factor that converts a
// dequantized MOS coordinate in the given unit to metres of the target
// projected CRS.
var unitsToMetres = [...]float64{
	UnitsMillimetres: 0.001,
	UnitsCentimetres: 0.01,
	UnitsDecimetres:  0.1,
	UnitsMetres:      1.0,
	UnitsKilometres:  1000.0,
}

// unitsNames holds the human-readable unit names.
var unitsNames = [...]string{
	UnitsMillimetres: "millimetres",
	UnitsCentimetres: "centimetres",
	UnitsDecimetres:  "decimetres",
	UnitsMetres:      "metres",
	UnitsKilometres:  "kilometres",
}

// ToMetres returns the factor converting coordinates expressed in these
// map units to metres. Returns an error for unsupported units
// (muGrad, muNone or any out-of-range value).
func (u MapUnits) ToMetres() (float64, error) {
	if u > UnitsKilometres {
		return 0, fmt.Errorf("mos: unsupported map units %d (only millimetres..kilometres are supported)", byte(u))
	}
	return unitsToMetres[u], nil
}

// String returns the human-readable unit name.
func (u MapUnits) String() string {
	if u > UnitsKilometres {
		return fmt.Sprintf("MapUnits(%d)", byte(u))
	}
	return unitsNames[u]
}

// systemInfoMinSize is the packed size of MapplGIS LayerInfo up to and
// including projectionSize:
// Version string[10] (11) + Precision (4) + flProjection (1) +
// reserved1 (4) + reserved2 (4) + reserved3 (2) + LayerID (4) +
// flLockForExport (1) + StylesIdentificationMode (1) + StyleLibraryID (4) +
// GridWidth (8) + GridHeight (8) + MapUnits (1) + flMapUnitsDefined (1) +
// reserved4[6] (6) + projectionSize (4) = 64.
const systemInfoMinSize = 64

// systemInfoVersion is the expected Version string[10] content (length
// prefix 5 + "Ver 1" + NUL padding).
var systemInfoVersion = []byte{5, 'V', 'e', 'r', ' ', '1'}

// IsSystemInfoBlob reports whether buf looks like a MapplGIS LayerInfo
// version wrapper rather than a regular MOS geometry blob: long enough to
// hold the fixed part and starting with the "Ver 1" version marker.
// Regular MOS blobs always start with an object type byte <= 4, so the
// marker cannot collide with real geometries.
func IsSystemInfoBlob(buf []byte) bool {
	if len(buf) < systemInfoMinSize || buf[0] != systemInfoVersion[0] {
		return false
	}
	return bytes.Equal(buf[:len(systemInfoVersion)], systemInfoVersion)
}

// SystemInfo holds the layer-level settings decoded from a
// MapplGIS LayerInfo blob.
type SystemInfo struct {
	// Precision is the number of decimal digits the quantized MOS
	// coordinates carry (kPrecision = 10^Precision).
	Precision int
	// MapUnits is the linear unit of the dequantized coordinates.
	MapUnits MapUnits
	// MapUnitsDefined reports whether the blob explicitly declares the map
	// unit. When no mos_units override is configured, the provider uses it to
	// convert decoded coordinates to metres.
	MapUnitsDefined bool
	// Projection is the raw PROJ.4 definition of the layer CRS, empty
	// when the layer carries none.
	Projection string
	// LayerID is the MapplGIS layer ID from the blob.
	LayerID int
}

// ParseSystemInfo decodes a MapplGIS LayerInfo version wrapper blob. The
// layout mirrors the packed MapplGIS record: Version string[10]
// (11 bytes), Precision int32 @11, flProjection byte @15, reserved1 int32
// @16, reserved2 int32 @20, reserved3 word @24, LayerID int32 @26,
// flLockForExport byte @30, StylesIdentificationMode byte @31,
// StyleLibraryID int32 @32, GridWidth float64 @36, GridHeight float64 @44,
// MapUnits byte @52, flMapUnitsDefined byte @53, reserved4[6] @54,
// projectionSize int32 @60, then projectionSize bytes of PROJ.4 text.
func ParseSystemInfo(buf []byte) (SystemInfo, error) {
	if !IsSystemInfoBlob(buf) {
		return SystemInfo{}, fmt.Errorf("mos: buffer is not a layer system info blob")
	}
	si := SystemInfo{
		Precision: int(int32(binary.LittleEndian.Uint32(buf[11:15]))),
		LayerID:   int(int32(binary.LittleEndian.Uint32(buf[26:30]))),
	}
	if flProjection := buf[15]; flProjection != 0 {
		size := int(int32(binary.LittleEndian.Uint32(buf[60:64])))
		if size < 0 || 64+size > len(buf) {
			return SystemInfo{}, fmt.Errorf("mos: invalid projection size %d (blob is %d bytes)", size, len(buf))
		}
		if size > 0 {
			si.Projection = strings.TrimSpace(string(buf[64 : 64+size]))
		}
	}
	si.MapUnits = MapUnits(buf[52])
	si.MapUnitsDefined = buf[53] != 0
	return si, nil
}

// ScaleToMetres returns the factor converting the declared units to metres.
// It returns 1 when the units are undefined.
func (si SystemInfo) ScaleToMetres() (float64, error) {
	if !si.MapUnitsDefined {
		return 1, nil
	}
	return si.MapUnits.ToMetres()
}

// ParseMapUnits parses the textual form accepted by the mysql provider's
// mos_units option. Short forms are intended for configuration files; the
// MapplGIS enum names and full English names are accepted as well.
func ParseMapUnits(value string) (MapUnits, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mm", "millimetre", "millimetres", "millimeter", "millimeters", "mumm":
		return UnitsMillimetres, nil
	case "cm", "centimetre", "centimetres", "centimeter", "centimeters", "musm":
		return UnitsCentimetres, nil
	case "dm", "decimetre", "decimetres", "decimeter", "decimeters", "mudm":
		return UnitsDecimetres, nil
	case "m", "metre", "metres", "meter", "meters", "mum":
		return UnitsMetres, nil
	case "km", "kilometre", "kilometres", "kilometer", "kilometers", "mukm":
		return UnitsKilometres, nil
	default:
		return 0, fmt.Errorf("unsupported MOS units %q (expected mm, cm, dm, m, km)", value)
	}
}
