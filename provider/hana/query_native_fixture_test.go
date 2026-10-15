package hana_test

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

// Native HANA requires collection envelope dimensionality to agree with its
// homogeneous children. Mixed literal children retain their original wire and
// are rejected by the actual constructor; no altitude is invented.
func hanaNativeFixtureWKB(g geom.Geometry) ([]byte, error) {
	collection, ok := g.(geom.Collection)
	if !ok {
		return querytest.EncodeFixtureWKB(g)
	}
	code := uint32(7)
	if hanaFixtureAllXYZ(g) {
		code = 1007
	}
	result := binary.LittleEndian.AppendUint32([]byte{1}, code)
	result = binary.LittleEndian.AppendUint32(result, uint32(len(collection)))
	for _, child := range collection {
		body, err := hanaNativeFixtureWKB(child)
		if err != nil {
			return nil, err
		}
		result = append(result, body...)
	}
	return result, nil
}

func hanaFixtureAllXYZ(g geom.Geometry) bool {
	if c, ok := g.(geom.Collection); ok {
		if len(c) == 0 {
			return false
		}
		for _, child := range c {
			if !hanaFixtureAllXYZ(child) {
				return false
			}
		}
		return true
	}
	switch g.(type) {
	case geom.PointZ, geom.LineStringZ, geom.PolygonZ, geom.MultiPointZ, geom.MultiLineStringZ, geometrycodec.MultiPolygonZ:
		return true
	}
	return false
}

func TestFeatureNativeFixtureCollectionHeaders(t *testing.T) {
	for _, c := range []struct {
		g    geom.Geometry
		code uint32
	}{
		{geom.Collection{geom.PointZ{1, 2, 3}, geom.Collection{geom.LineStringZ{}}}, 1007},
		{geom.Collection{geom.Point{1, 2}, geom.PointZ{1, 2, 3}}, 7},
	} {
		wire, err := hanaNativeFixtureWKB(c.g)
		if err != nil {
			t.Fatal(err)
		}
		if binary.LittleEndian.Uint32(wire[1:5]) != c.code {
			t.Fatal("wrong collection header")
		}
		actual, err := geometrycodec.DecodeRawWKB(wire)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, c.g) {
			t.Fatalf("literal changed: %#v", actual)
		}
	}
}
