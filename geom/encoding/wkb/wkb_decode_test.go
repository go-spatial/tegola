package wkb_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom/encoding/wkb"
	"github.com/go-spatial/tegola/geom/encoding/wkb/internal/tcase"
)

func TestWKBDecode(t *testing.T) {
	fnames, err := tcase.GetFiles("testdata")
	if err != nil {
		t.Fatalf("error getting files: %v", err)
	}
	var fname string

	fn := func(t *testing.T, tc tcase.C) {
		if tc.Skip.Is(tcase.TypeDecode) {
			t.Skip("instructed to skip.")
		}
		geom, err := wkb.DecodeBytes(tc.Bytes)
		if !tc.DoesErrorMatch(tcase.TypeDecode, err) {
			eerr := "nil"
			if tc.HasErrorFor(tcase.TypeDecode) {
				eerr = tc.ErrorFor(tcase.TypeDecode)
			}
			t.Errorf("error, expected %v got %v", eerr, err)
			return
		}
		if tc.HasErrorFor(tcase.TypeDecode) {
			return
		}
		if !reflect.DeepEqual(geom, tc.Expected) {
			t.Errorf("decode, expected %v got %v", tc.Expected, geom)
		}
	}

	for _, fname = range fnames {
		cases, err := tcase.ParseFile(fname)
		if err != nil {
			t.Fatalf("error parsing file: %v : %v ", fname, err)
			continue
		}
		t.Run(fname, func(t *testing.T) {
			if len(cases) == 1 {
				t.Logf("found one test case in %v ", fname)
			} else {

				t.Logf("found %2v test cases in %v ", len(cases), fname)
			}
			for i := range cases {
				t.Run(cases[i].Desc, func(t *testing.T) { fn(t, cases[i]) })
			}
		})
	}
}
