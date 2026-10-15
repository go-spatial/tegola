package geometrycodec

import (
	"bytes"
	"reflect"
	"testing"
)

func FuzzRawWKBSecurity(f *testing.F) {
	for _, b := range [][]byte{{}, {0}, {1, 1, 0, 0, 0}, {1, 7, 0, 0, 0, 255, 255, 255, 255}, {1, 2, 0, 0, 0, 255, 255, 255, 255}} {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		before := append([]byte(nil), data...)
		g, err := DecodeRawWKB(data)
		again, e := DecodeRawWKB(data)
		if (err == nil) != (e == nil) || err == nil && !reflect.DeepEqual(g, again) {
			t.Fatal("nondeterministic decoder")
		}
		if !bytes.Equal(before, data) {
			t.Fatal("decoder mutated source")
		}
	})
}
func FuzzRawWKTSecurity(f *testing.F) {
	for _, s := range []string{"", "POINT(1 2)", "POINT Z(1 2 3)", "GEOMETRYCOLLECTION(POINT EMPTY,LINESTRING EMPTY)", "POLYGON((0 0,1 0,1 1,0 0))", "POINT(NaN 0)", "\xff", "GEOMETRYCOLLECTION(GEOMETRYCOLLECTION("} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		g, err := DecodeRawWKT(text)
		again, e := DecodeRawWKT(text)
		if (err == nil) != (e == nil) || err == nil && !reflect.DeepEqual(g, again) {
			t.Fatal("nondeterministic decoder")
		}
	})
}
func FuzzMOSSecurity(f *testing.F) {
	for _, b := range [][]byte{{}, {0}, {1, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {1, 0, 0, 0, 255, 255, 255, 255, 255, 127}} {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		before := append([]byte(nil), data...)
		g, err := DecodeMOS(data, DefaultMOSConfig())
		again, e := DecodeMOS(data, DefaultMOSConfig())
		if (err == nil) != (e == nil) || err == nil && !reflect.DeepEqual(g, again) {
			t.Fatal("nondeterministic decoder")
		}
		if !bytes.Equal(before, data) {
			t.Fatal("decoder mutated source")
		}
	})
}
