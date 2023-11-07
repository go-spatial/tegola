package atlas_test

import (
	"strings"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider/test"
)

var testLayer1 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-1",
	MinZoom:           4,
	MaxZoom:           9,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Point{},
	DefaultTags: env.Dict{
		"foo": "bar",
	},
}

var testLayer2 = atlas.Layer{
	Name:              "test-layer-2-name",
	ProviderLayerName: "test-layer-2-provider-layer-name",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &test.TileProvider{},
	GeomType:          geom.LineString{},
	DefaultTags: env.Dict{
		"foo": "bar",
	},
}

var testLayer3 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-3",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Point{},
	DefaultTags:       env.Dict{},
}

var testMap = atlas.Map{
	Name:        "test-map",
	Attribution: "test attribution",
	Center:      [3]float64{1.0, 2.0, 3.0},
	Layers: []atlas.Layer{
		testLayer1,
		testLayer2,
		testLayer3,
	},
}

func TestAddMaps(t *testing.T) {
	a := &atlas.Atlas{}

	// Should initialize from empty
	maps := []atlas.Map{
		{Name: "First Map"},
		{Name: "Second Map"},
	}
	err := a.AddMaps(maps)
	if err != nil {
		t.Errorf("Unexpected error when addings maps. %s", err)
	}

	m, err := a.Map("Second Map")
	if err != nil {
		t.Errorf("Failed retrieving map from Atlas. %s", err)
	} else if m.Name != "Second Map" {
		t.Errorf("Expected map named \"Second Map\". Found %v.", m)
	}

	// Should error if duplicate name.
	err = a.AddMaps([]atlas.Map{{Name: "First Map"}})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Should return error for duplicate map name. err=%s", err)
	}
}

func TestRemoveMaps(t *testing.T) {
	a := &atlas.Atlas{}
	a.AddMaps([]atlas.Map{
		{Name: "First Map"},
		{Name: "Second Map"},
	})

	if len(a.AllMaps()) != 2 {
		t.Error("Unexpected failure setting up Atlas. No maps added.")
		return
	}

	a.RemoveMaps([]string{"Second Map"})
	maps := a.AllMaps()
	if len(maps) != 1 || maps[0].Name == "Second Map" {
		t.Error("Should have deleted \"Second Map\". Didn't.")
	}
}
