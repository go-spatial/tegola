package wkb_test

import (
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/geom/encoding/wkb"
	"github.com/go-spatial/tegola/geom/encoding/wkb/internal/tcase"
)

func TestWKBDecode(t *testing.T) {
	fnames, err := tcase.GetFiles("testdata")
	if err != nil {
		t.Fatalf("error getting files: %v", err)
	}
	var fname string
	// log.Println("Got the following files:", fnames)

	fn := func(idx int, tcase tcase.C) {
		geom, err := wkb.DecodeBytes(tcase.Bytes)
		if err != nil {
			t.Errorf("[%v:%v] Error, Expected nil Got %v", fname, idx, err)
			return
		}
		if !reflect.DeepEqual(geom, tcase.Expected) {
			t.Errorf("[%v:%v]  %v did not get decoded correctly, \n\tExpected  %v \n\tGot       %v", fname, idx, tcase.Desc, tcase.Expected, geom)
		}
	}

	for _, fname = range fnames {
		cases, err := tcase.ParseFile(fname)
		if err != nil {
			t.Fatalf("error parsing file: %v : %v ", fname, err)
		}
		if len(cases) == 1 {
			t.Logf("found one test case in %v ", fname)
		} else {

			t.Logf("found %2v test cases in %v ", len(cases), fname)
		}
		//t.Logf(cases)
		var tcases []tbltest.TestCase
		for i := range cases {
			tcases = append(tcases, cases[i])
		}
		tbltest.Cases(tcases...).Run(fn)
	}
}
