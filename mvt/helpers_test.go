package mvt

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

func fromPixel(x, y float64) (*basic.Point, error) {
	pt, err := tile.FromPixel(tegola.WebMercator, [2]float64{x, y})
	if err != nil {
		return nil, err
	}
	bpt := basic.Point(pt)
	return &bpt, nil
}
