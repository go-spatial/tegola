package gpkg

import (
	"encoding/binary"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func fmt8Bit(n byte) string {
	val := []byte("0b00000000")
	s := strconv.FormatUint(uint64(n), 2)
	offset := len(val) - len(s)
	copy(val[offset:], []byte(s))
	return string(val)
}

func TestHeaderFlag(t *testing.T) {

	tcheader := func(hf headerFlags) string { return hf.String() + " " + fmt8Bit(byte(hf)) }

	type tcase struct {
		header     headerFlags
		IsEmpty    bool
		IsStandard bool
		Envelope   envelopeType
		Endian     binary.ByteOrder
	}

	fn := func(tc tcase) (string, func(*testing.T)) {
		return tcheader(tc.header), func(t *testing.T) {
			t.Parallel()
			if tc.header.IsEmpty() != tc.IsEmpty {
				t.Errorf("is empty, expected %v got %v", tc.IsEmpty, tc.header.IsEmpty())
			}
			if tc.header.IsStandard() != tc.IsStandard {
				t.Errorf("is standard, expected %v got %v", tc.IsStandard, tc.header.IsStandard())
			}
			if tc.header.Endian() != tc.Endian {
				t.Errorf("byte order, expected %v got %v", tc.Endian, tc.header.Endian())
			}
			if tc.header.Envelope() != tc.Envelope {
				t.Errorf("envelope type, expected %v got %v", tc.Envelope, tc.header.Envelope())
			}
			// The following two tests are just for coverage. :P The bottom two functions are of course
			// will always pass if the above one passes. The one that really needs to be tested for is
			// the NumberOfElements, but that is tested with binaryheader and is really just a table lookup.
			if tc.header.Envelope().String() != tc.Envelope.String() {
				t.Errorf("envelope type name, expected %v got %v", tc.Envelope, tc.header.Envelope())
			}
			if tc.header.Envelope().NumberOfElements() != tc.Envelope.NumberOfElements() {
				t.Errorf("envelope type name, expected %v got %v", tc.Envelope.NumberOfElements(), tc.header.Envelope().NumberOfElements())
			}
		}
	}

	tests := [...]tcase{
		{
			header:     0x00, // 00 0 0 000 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x01, // 00 0 0 000 1
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x02, // 00 0 0 001 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeXY,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x03, // 00 0 0 001 1
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeXY,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x04, // 00 0 0 010 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeXYZ,
			Endian:     binary.BigEndian,
		},

		// 0x05 is the same as 0x04 but LittleEndian

		{
			header:     0x06, // 00 0 0 011 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeXYM,
			Endian:     binary.BigEndian,
		},
		// 0x07 is the same as 0x06 but Little Endian
		{
			header:     0x08, // 00 0 0 100 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeXYZM,
			Endian:     binary.BigEndian,
		},
		// 0x09 is the same as 0x08 but little Endian
		{
			header:     0x0A, // 00 0 0 101 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x0B, // 00 0 0 101 1
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x0C, // 00 0 0 110 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.BigEndian,
		},
		// 0x0D is the same as 0x0C but little Endian
		{
			header:     0x0E, // 00 0 0 111 0
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x0F, // 00 0 0 111 1
			IsStandard: true,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x10, // 00 0 1 000 0
			IsStandard: true,
			IsEmpty:    true,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.BigEndian,
		},
		// 0x11-0x1F are the various iterations of 0x02-0x0F but with IsEmpty bit set to true
		{
			header:     0x20, // 00 1 0 000 0
			IsStandard: false,
			IsEmpty:    false,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.BigEndian,
		},
		// 0x21-0x2F are the various iterations of 0x02-0x0F but with IsExtention bit set to true
		// No need to test the high bits 0x30-0xFF as the are reserved.
	}
	for _, tc := range tests {
		t.Run(fn(tc))
	}
}

func TestBinaryHeader(t *testing.T) {
	type tcase struct {
		bytes        []byte
		version      uint8
		flags        headerFlags // This will not be tested as it's already being tested elsewhere.
		srsid        int32
		envelopetype envelopeType
		envelope     []float64
		size         int
		empty        bool
		standard     bool
		err          error
	}
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			var bh *BinaryHeader
			var err error
			if tc.bytes != nil {
				bh, err = NewBinaryHeader(tc.bytes)
			}
			if tc.err != nil {
				if err == nil {
					t.Errorf("error, expected %v got nil", tc.err)
					return
				}
				if tc.err.Error() != err.Error() {
					t.Errorf("error, expected %v got %v", tc.err.Error(), err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("error, expected nil got %v", err.Error())
				return

			}

			if bh.Version() != tc.version {
				t.Errorf("version, expected %v got %v", tc.version, bh.Version())
			}
			if bh.SRSId() != tc.srsid {
				t.Errorf("SRS Id, expected %v got %v", tc.srsid, bh.SRSId())
			}
			if bh.EnvelopeType() != tc.envelopetype {
				t.Errorf("envelope type, expected %v got %v", tc.envelopetype, bh.EnvelopeType())
			}
			if !reflect.DeepEqual(bh.Envelope(), tc.envelope) {
				t.Errorf("envelope, expected %v got %v", tc.envelope, bh.Envelope())
			}
			if !reflect.DeepEqual(bh.Magic(), Magic) {
				t.Errorf("magic, expected %v got %v", Magic, bh.Magic())
			}
			if bh.IsGeometryEmpty() != tc.empty {
				t.Errorf("empty geometry, expected %v got %v", tc.empty, bh.IsGeometryEmpty())
			}
			if bh.IsStandardGeometry() != tc.standard {
				t.Errorf("standard geometry, expected %v got %v", tc.standard, bh.IsStandardGeometry())
			}
			if bh.Size() != tc.size {
				t.Errorf("header size, expected %v got %v", tc.size, bh.Size())
			}
		}
	}
	tests := map[string]tcase{
		"nil": {
			empty:        true,
			standard:     true,
			envelopetype: EnvelopeTypeInvalid,
		},
		"zero bytes": {
			bytes: []byte{},
			err:   errors.New("not enough bytes to decode header"),
		},
		"bad magic": {
			bytes: []byte{
				0x50, 0x47, // Magic number
				0x00,                   // Version
				0x03,                   // Flags -- LittleEndian, XY
				0xE6, 0x10, 0x00, 0x00, // srs_id
				0xE5, 0x6D, 0xFA, 0xB6, 0x67, 0xB6, 0x37, 0x40, // MinX
				0xC1, 0xAB, 0xB0, 0xD0, 0xB9, 0xCB, 0x37, 0x40, // MaxX
				0x2C, 0xC9, 0xBC, 0xE5, 0xD6, 0xF2, 0x42, 0x40, // MinY
				0x20, 0xC2, 0x2E, 0x86, 0xB8, 0xF8, 0x42, 0x40, // MaxY
			},
			err: errors.New("invalid magic number"),
		},
		"invalid envelope XY given for XYZM": {
			bytes: []byte{
				0x47, 0x50, // Magic number
				0x00,                   // Version
				0x09,                   // Flags -- LittleEndian, XY
				0xE6, 0x10, 0x00, 0x00, // srs_id
				0xE5, 0x6D, 0xFA, 0xB6, 0x67, 0xB6, 0x37, 0x40, // MinX
				0xC1, 0xAB, 0xB0, 0xD0, 0xB9, 0xCB, 0x37, 0x40, // MaxX
				0x2C, 0xC9, 0xBC, 0xE5, 0xD6, 0xF2, 0x42, 0x40, // MinY
				0x20, 0xC2, 0x2E, 0x86, 0xB8, 0xF8, 0x42, 0x40, // MaxY
			},
			err: errors.New("not enough bytes to decode header"),
		},
		"invalid envelope type": {
			bytes: []byte{
				0x47, 0x50, // Magic number
				0x00,                   // Version
				0x0B,                   // Flags -- LittleEndian, XY
				0xE6, 0x10, 0x00, 0x00, // srs_id
				0xE5, 0x6D, 0xFA, 0xB6, 0x67, 0xB6, 0x37, 0x40, // MinX
				0xC1, 0xAB, 0xB0, 0xD0, 0xB9, 0xCB, 0x37, 0x40, // MaxX
				0x2C, 0xC9, 0xBC, 0xE5, 0xD6, 0xF2, 0x42, 0x40, // MinY
				0x20, 0xC2, 0x2E, 0x86, 0xB8, 0xF8, 0x42, 0x40, // MaxY
			},
			err: errors.New("invalid envelope type"),
		},
		"4326 XY": {
			bytes: []byte{
				0x47, 0x50, // Magic number
				0x00,                   // Version
				0x03,                   // Flags -- LittleEndian, XY
				0xE6, 0x10, 0x00, 0x00, // srs_id
				0xE5, 0x6D, 0xFA, 0xB6, 0x67, 0xB6, 0x37, 0x40, // MinX
				0xC1, 0xAB, 0xB0, 0xD0, 0xB9, 0xCB, 0x37, 0x40, // MaxX
				0x2C, 0xC9, 0xBC, 0xE5, 0xD6, 0xF2, 0x42, 0x40, // MinY
				0x20, 0xC2, 0x2E, 0x86, 0xB8, 0xF8, 0x42, 0x40, // MaxY
			},
			version:      0,
			flags:        headerFlags(0x03),
			srsid:        4326,
			envelopetype: EnvelopeTypeXY,
			envelope: []float64{
				23.712520061626396, 23.79580406487708,
				37.89718314855631, 37.94313123019333,
			},
			size:     40,
			empty:    false,
			standard: true,
			err:      nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
