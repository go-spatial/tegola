// Copyright 2015 Dmitry Vyukov. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package main

import (
	"github.com/dvyukov/go-fuzz/gen"

	"github.com/go-spatial/tegola/geom/encoding/wkb/internal/tcase"
)

func main() {
	createFromTestData()
}

func createFromTestData() {
	fnames, _ := tcase.GetFiles("testdata")
	var fname string

	for _, fname = range fnames {
		cases, _ := tcase.ParseFile(fname)
		for i := range cases {
			gen.Emit(cases[i].Bytes, nil, true)
		}
	}

}
