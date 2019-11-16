// +build cgo

package mbtiles_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache/mbtiles"
	"github.com/go-spatial/tegola/dict"
)

func TestNew(t *testing.T) {
	type tcase struct {
		config   dict.Dict
		expected *mbtiles.Cache
		err      error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		output, err := mbtiles.New(tc.config)
		if err != nil {

			if tc.err != nil && err.Error() == tc.err.Error() {
				// correct error returned
				return
			}
			t.Errorf("unexpected error %v", err)
			return
		}

		if !cmp.Equal(tc.expected, output, cmpopts.IgnoreUnexported(mbtiles.Cache{})) { //Reflect compare un-exported field like dbList
			t.Errorf("expected %+v got %+v", tc.expected, output)
			return
		}
	}

	tests := map[string]tcase{
		"valid basepath": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   "-180.0,-85,180,85",
				MinZoom:  0,
				MaxZoom:  tegola.MaxZ,
			},
			err: nil,
		},
		"valid basepath and max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(9),
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   "-180.0,-85,180,85",
				MinZoom:  0,
				MaxZoom:  9,
			},
			err: nil,
		},
		"valid basepath and min zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(2),
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   "-180.0,-85,180,85",
				MinZoom:  0,
				MaxZoom:  2,
			},
			err: nil,
		},
		"valid basepath and bounds": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"bounds":   "-180.0, -85.0511, 180.0, 85.0511",
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   "-180.0, -85.0511, 180.0, 85.0511", //TODO should be cleaned
				MinZoom:  0,
				MaxZoom:  tegola.MaxZ,
			},
			err: nil,
		},
		"missing basepath": {
			config:   map[string]interface{}{},
			expected: nil,
			err:      mbtiles.ErrMissingBasepath,
		},
		"invalid zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": "foo",
			},
			expected: nil,
			err:      fmt.Errorf(`config: value mapped to "max_zoom" is string not uint`),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
