package gpkg

import (
	"fmt"
	"testing"

	"github.com/terranodo/tegola/internal/assert"
)

var geometryHeader = []byte{71, 80, 0, 3, 230, 16, 0, 0, 229, 109, 250, 182, 103, 182, 55, 64, 193, 171, 176, 208, 185, 203, 55, 64, 44, 201, 188, 229, 214, 242, 66, 64, 32, 194, 46, 134, 184, 248, 66, 64}

func TestBytesToFloat64(t *testing.T) {
	type TestCase struct {
		data      [8]byte
		byteOrder uint8
		expected  float64
	}

	testCases := []TestCase{
		{
			data:      [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
			byteOrder: wkbXDR,
			expected:  0.0,
		},
		{
			data:      [8]byte{64, 64, 62, 86, 4, 24, 147, 117},
			byteOrder: wkbXDR,
			expected:  32.487,
		},
		{
			data:      [8]byte{64, 84, 159, 43, 2, 12, 73, 186},
			byteOrder: wkbXDR,
			expected:  82.487,
		},
		{
			data:      [8]byte{63, 240, 0, 0, 0, 0, 0, 0},
			byteOrder: wkbXDR,
			expected:  1,
		},
		{
			data:      [8]byte{64, 248, 106, 8, 0, 0, 0, 0},
			byteOrder: wkbXDR,
			expected:  100000.5,
		},
	}

	for i, tc := range testCases {
		f64Value := bytesToFloat64(tc.data[:], tc.byteOrder)
		if f64Value != tc.expected {
			msg := fmt.Sprintf("TestCase[%v] - Value doesn't match expected.", i)
			assert.Equal(t, tc.expected, f64Value, msg)
		}
	}
}

func TestBytesToInt32(t *testing.T) {
	type TestCase struct {
		data      [4]byte
		byteOrder uint8
		expected  int32
	}

	testCases := []TestCase{
		{
			data:      [4]byte{0, 0, 0, 0},
			byteOrder: wkbNDR,
			expected:  0,
		},
		{
			data:      [4]byte{230, 16, 0, 0},
			byteOrder: wkbNDR,
			expected:  4326,
		},
	}

	for i, tc := range testCases {
		i32value := bytesToInt32(tc.data[:], tc.byteOrder)
		msg := fmt.Sprint("TestCase[%v] - Value doesn't match expected.", i)
		assert.Equal(t, i32value, tc.expected, msg)
	}
}

func TestGeoPackageBinaryHeaderInit(t *testing.T) {
	initializedExpected := true
	var magicExpected uint16 = 0x4750
	var versionExpected uint8 = 0x0
	var flagsExpected uint8 = 0x3
	var srs_idExpected int32 = 4326
	envelopeExpected := []float64{23.712520061626396, 23.79580406487708, 37.89718314855631, 37.94313123019333}
	sizeExpected := 40

	var h GeoPackageBinaryHeader
	h.Init(geometryHeader)

	assert.Equal(t, initializedExpected, h.isInitialized("TestInit"), "")
	assert.Equal(t, magicExpected, h.Magic(), "")
	assert.Equal(t, versionExpected, h.Version(), "")
	assert.Equal(t, flagsExpected, h.flags, "")
	assert.Equal(t, srs_idExpected, h.SRSId(), "")
	assert.Equal(t, envelopeExpected, h.Envelope(), "")
	assert.Equal(t, sizeExpected, h.headerSize)
}

func TestEnvelopeType(t *testing.T) {
	/* The envelope is a 3-bit unsiged integer composed of the flag bits 1-3
	 */
	var h GeoPackageBinaryHeader
	h.flagsReady = true

	// The first byte here shouldn't make any difference
	testCases := map[byte]uint8{
		0xFF: 7,
		0xFE: 7,
		0x3D: 6,
		0x8C: 6,
		0x3B: 5,
		0x2A: 5,
		0x19: 4,
		0x18: 4,
		0x77: 3,
		0x76: 3,
		0x65: 2,
		0x64: 2,
		0x53: 1,
		0x52: 1,
	}

	for flags, expectedEnvType := range testCases {
		h.flags = flags
		envType := h.EnvelopeType()
		assert.Equal(t, envType, expectedEnvType, "")
	}
}
