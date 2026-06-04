package provider_test

import (
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/provider/test"
)

func TestProviderInterface(t *testing.T) {
	var (
		stdName = provider.TypeStd.Prefix() + test.Name
		mvtName = provider.TypeMvt.Prefix() + test.Name
	)
	if _, err := provider.For(stdName, nil, nil); err != nil {
		t.Errorf("retrieve provider err , expected nil got %v", err)
		return
	}
	if test.Count != 1 {
		t.Errorf(" expected count , expected 1 got %v", test.Count)
	}
	provider.Cleanup()
	if test.Count != 0 {
		t.Errorf(" expected count , expected 0 got %v", test.Count)
	}
	if _, err := provider.For(mvtName, nil, nil); err != nil {
		t.Errorf("retrieve provider err , expected nil got %v", err)
		return
	}
	if test.MVTCount != 1 {
		t.Errorf(" expected count , expected 1 got %v", test.MVTCount)
	}
	provider.Cleanup()
	if test.MVTCount != 0 {
		t.Errorf(" expected count , expected 0 got %v", test.MVTCount)
	}
}

func TestNewTileWorldCRS84QuadExtent(t *testing.T) {
	tile := provider.NewTile(0, 1, 0, 64, tegola.WGS84)
	ext, srid := tile.Extent()
	if srid != tegola.WGS84 {
		t.Fatalf("srid, expected %d got %d", tegola.WGS84, srid)
	}
	if got, expected := ext.Extent(), [4]float64{0, -90, 180, 90}; got != expected {
		t.Fatalf("extent, expected %v got %v", expected, got)
	}
}
