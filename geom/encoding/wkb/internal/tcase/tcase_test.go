package tcase

import (
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/geom"
)

type testcase struct {
	filename string
	cases    []C
}

func TestParse(t *testing.T) {

	fn := func(idx int, test testcase) {
		cases, err := ParseFile(test.filename)
		if err != nil {
			t.Errorf("%v : expect error: nil got: %v", idx, err)
		}
		if len(cases) != len(test.cases) {
			t.Errorf("%v : number of cases expected: %v got: %v", idx, len(test.cases), len(cases))
			return
		}
		for i, tcase := range test.cases {
			acase := cases[i]
			if acase.Desc != tcase.Desc {
				t.Errorf("%v : Desc expected: %v got: %v", idx, tcase.Desc, acase.Desc)
			}
			if !reflect.DeepEqual(tcase.Expected, acase.Expected) {
				t.Errorf("%v : Expected expected: %#v got: %#v", idx, tcase.Expected, acase.Expected)
			}
			if !reflect.DeepEqual(tcase.Bytes, acase.Bytes) {
				t.Errorf("%v : Bytes expected: %v got: %v", idx, tcase.Bytes, acase.Bytes)
			}
		}
	}
	tbltest.Cases(
		testcase{
			filename: "testdata/point.tcase",
			cases: []C{
				{
					Desc:     "This is a simple test",
					Expected: geom.Point{2, 4},
					Bytes:    []byte{0x00, 0x00, 0x00, 0x00, 0xaf, 0x00, 0xaf, 0x0c, 0xd0, 0x0d, 0xac, 0x00, 0xDE, 0xAF, 0xD0, 0x0d, 0xac, 0xff, 0x00, 0x00},
				},
				{
					Desc:     "This is a simple test",
					Expected: geom.Point{2, 4},
					Bytes:    []byte{0x00, 0x00, 0x00, 0x00, 0xaf, 0x00, 0xaf, 0x0c, 0xd0, 0x0d, 0xac, 0x00, 0xDE, 0xAF, 0xD0, 0x0d, 0xac, 0xff, 0x00, 0x00},
				},
			},
		},
	).Run(fn)

}
