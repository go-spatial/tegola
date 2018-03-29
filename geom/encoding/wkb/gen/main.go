// Copyright 2015 Dmitry Vyukov. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package main

import (
	"bytes"
	"github.com/dvyukov/go-fuzz/gen"

	"github.com/go-spatial/tegola/geom/encoding/wkb/internal/tcase"
)

func main() {
	createFromTestData()
}

func createFromTestData() []byte {
	fnames, err := tcase.GetFiles("testdata")
	if err != nil {
		t.Fatalf("error getting files: %v", err)
	}
	var fname string

	fn := func(idx int, tc tcase.C) {
		gen.Emit(tc.Expected, nil, true)
	}

	for _, fname = range fnames {
		cases, err := tcase.ParseFile(fname)
		if err != nil {
			t.Fatalf("error parsing file: %v : %v ", fname, err)
		}
		for i := range cases {
			gen.Emit(cases[i].Expected, nil, true)
		}
	}

}
