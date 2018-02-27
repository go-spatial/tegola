package wkb_test

import (
	"log"
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/geom/encoding/wkb"
	"github.com/go-spatial/tegola/geom/encoding/wkb/internal/tcase"
)

func TestWKBEncode(t *testing.T) {
	fnames, err := tcase.GetFiles("testdata")
	if err != nil {
		t.Fatalf("error getting files: %v", err)
	}
	var fname string

	fn := func(idx int, tc tcase.C) {
		bs, err := wkb.EncodeBytes(tc.Expected)
		if err != nil {
			log.Println("TestCase:", tc)
			t.Errorf("[%v:%v] Error, Expected nil Got %v", fname, idx, err)
			return
		}
		if !reflect.DeepEqual(bs, tc.Bytes) {
			t.Errorf("[%v:%v] %v did not encoded geometry correctly, \n\tExpected\n%v \n\tGot\n%v", fname, idx, tc.Desc, tcase.SprintBinary(tc.Bytes, "\t"), tcase.SprintBinary(bs, "\t"))
		}
	}

	for _, fname = range fnames {
		cases, err := tcase.ParseFile(fname)
		if err != nil {
			t.Fatalf("error parsing file: %v : %v ", fname, err)
		}
		if len(cases) == 1 {
			t.Logf("found one test case in %v", fname)
		} else {
			t.Logf("found %2v test cases in %v", len(cases), fname)

		}
		// t.Logf(cases)
		var tcases []tbltest.TestCase
		for i := range cases {
			tcases = append(tcases, cases[i])
		}
		tbltest.Cases(tcases...).Run(fn)
	}
}
