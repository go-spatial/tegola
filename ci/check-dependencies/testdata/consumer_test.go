package consumer

import (
	"context"
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	vectorTile "github.com/alexeydott/geom/encoding/mvt/vector_tile"
	"github.com/alexeydott/tegola/basic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestForkMVT(t *testing.T) {
	options := vectorTile.File_vector_tile_proto.Options().(*descriptorpb.FileOptions)
	if options.GetGoPackage() != "github.com/alexeydott/geom/encoding/mvt/vector_tile;vectorTile" {
		t.Fatalf("unexpected generated protobuf package: %s", options.GetGoPackage())
	}
	feature := mvt.Feature{Geometry: geom.LineString{{1, 2}}}
	tileFeature, err := feature.VTileFeature(context.Background(), nil, nil)
	if err != nil || tileFeature != nil {
		t.Fatalf("degenerate line: got %v, %v; want nil, nil", tileFeature, err)
	}
	if _, err := proto.Marshal(&vectorTile.Tile{}); err != nil {
		t.Fatalf("protobuf APIv2: %v", err)
	}
}

func TestForkDatumShift(t *testing.T) {
	code, err := basic.RegisterProj4Defn("+proj=etmerc +ellps=bessel +towgs84=41,-107.6,-93,0,0,0,0 +x_0=0 +y_0=0 +lon_0=37.5 +k_0=1 +lat_0=55.6666666667 +units=m +no_defs")
	if err != nil {
		t.Fatal(err)
	}
	converted, err := basic.ToWebMercator(code, geom.Point{769.792, 19300.763})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := converted.(geom.Point)
	if !ok {
		t.Fatalf("unexpected geometry type %T", converted)
	}
	// Independent PROJ reference, also used by the root regression test.
	want := geom.Point{4175652.829, 7526703.655}
	if math.IsNaN(got[0]) || math.IsNaN(got[1]) || math.Abs(got[0]-want[0]) > 0.1 || math.Abs(got[1]-want[1]) > 0.1 {
		t.Fatalf("datum shift: got %v, want %v within 0.1 metre", got, want)
	}
}

func TestGeographicForkDatumShift(t *testing.T) {
	code, err := basic.RegisterProj4Defn("+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93")
	if err != nil {
		t.Fatal(err)
	}
	converted, err := basic.ToWebMercator(code, geom.Point{37.6, 55.7})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := converted.(geom.Point)
	want := geom.Point{4185417.6034322004, 7498990.087942868}
	if !ok || math.IsNaN(got[0]) || math.IsNaN(got[1]) || math.Abs(got[0]-want[0]) > 0.001 || math.Abs(got[1]-want[1]) > 0.001 {
		t.Fatalf("geographic datum: got %v, want %v", converted, want)
	}
}
