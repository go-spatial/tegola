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

	tcheader := func(hf headerFlag) string { return hf.String() + " " + fmt8Bit(byte(hf)) }

	type tcase struct {
		header     headerFlag
		IsEmpty    bool
		IsStandard bool
		Envelope   envelopeType
		Endian     binary.ByteOrder
	}

	fn := func(t *testing.T, tc tcase) {
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

	tests := []tcase{
		{
			header:     0x00,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x01,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x02,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeXY,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x03,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeXY,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x04,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeXYZ,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x06,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeXYM,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x08,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeXYZM,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x0A,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x0B,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.LittleEndian,
		},
		{
			header:     0x0C,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x0E,
			IsEmpty:    false,
			IsStandard: true,
			Envelope:   EnvelopeTypeInvalid,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x10,
			IsEmpty:    true,
			IsStandard: true,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.BigEndian,
		},
		{
			header:     0x20,
			IsEmpty:    false,
			IsStandard: false,
			Envelope:   EnvelopeTypeNone,
			Endian:     binary.BigEndian,
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(tcheader(tests[i].header), func(t *testing.T) { fn(t, test) })
	}
}

func TestBinaryHeader(t *testing.T) {
	type tcase struct {
		bytes        []byte
		version      uint8
		flags        headerFlag // This will not be tested as it's already being tested elsewhere.
		srsid        int32
		envelopetype envelopeType
		envelope     []float64
		size         int
		empty        bool
		standard     bool
		err          error
	}
	fn := func(t *testing.T, tc tcase) {
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
	tests := map[string]tcase{
		"nil": tcase{
			empty:        true,
			standard:     true,
			envelopetype: EnvelopeTypeInvalid,
		},
		"zero bytes": tcase{
			bytes: []byte{},
			err:   errors.New("not enough bytes to decode header"),
		},
		"bad magic": tcase{
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
		"invalid envelope XY given for XYZM": tcase{
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
		"invalid envelope type": tcase{
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
		"4326 XY": tcase{
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
			flags:        headerFlag(0x03),
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
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
