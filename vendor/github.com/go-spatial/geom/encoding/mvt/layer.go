package mvt

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"context"

	vectorTile "github.com/go-spatial/geom/encoding/mvt/vector_tile"
)

// Layer describes a layer within a tile.
// Each layer can have multiple features
type Layer struct {
	// Name is the unique name of the layer within the tile
	Name string
	// The set of features
	features []Feature
	// default is 4096
	extent *int
}

func valMapToVTileValue(valMap []interface{}) (vt []*vectorTile.Tile_Value) {
	for _, v := range valMap {
		vt = append(vt, vectorTileValue(v))
	}

	return vt
}

// VTileLayer returns a vectorTile Tile_Layer object that represents this layer.
func (l *Layer) VTileLayer(ctx context.Context) (*vectorTile.Tile_Layer, error) {
	kmap, vmap, err := keyvalMapsFromFeatures(l.features)
	if err != nil {
		return nil, err
	}

	valmap := valMapToVTileValue(vmap)

	var features = make([]*vectorTile.Tile_Feature, 0, len(l.features))
	for _, f := range l.features {
		// context check
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		vtf, err := f.VTileFeature(ctx, kmap, vmap)
		if err != nil {
			switch err {
			case context.Canceled:
				return nil, err
			default:
				return nil, fmt.Errorf("error getting VTileFeature: %v", err)
			}
		}

		if vtf != nil {
			features = append(features, vtf)
		}
	}

	ext := uint32(l.Extent())
	version := uint32(l.Version())
	vtl := new(vectorTile.Tile_Layer)
	vtl.Version = &version
	name := l.Name // Need to make a copy of the string.
	vtl.Name = &name
	vtl.Features = features
	vtl.Keys = kmap
	vtl.Values = valmap
	vtl.Extent = &ext

	return vtl, nil
}

// Version is the version of tile spec this layer is from.
func (*Layer) Version() int { return int(Version) }

// Extent defaults to 4096
func (l *Layer) Extent() int {
	if l == nil || l.extent == nil {
		return int(DefaultExtent)
	}
	return *(l.extent)
}

// SetExtent sets the extent value
func (l *Layer) SetExtent(e int) {
	if l == nil {
		return
	}
	l.extent = &e
}

// Features returns a copy of the features in the layer, use the index of the this
// array to remove any features from the layer
func (l *Layer) Features() (f []Feature) {
	if l == nil || l.features == nil {
		return nil
	}
	f = append(f, l.features...)
	return f
}

// AddFeatures will add one or more Features to the Layer
// per the spec features SHOULD have unique ids but it's not required
func (l *Layer) AddFeatures(features ...Feature) {
	// pre allocate memory
	b := make([]Feature, len(l.features)+len(features))

	copy(b, l.features)
	copy(b[len(l.features):], features)

	l.features = b
}

// RemoveFeature allows you to remove one or more features, with the provided indexes.
// To figure out the indexes, use the indexs from the Features array.
func (l *Layer) RemoveFeature(idxs ...int) {
	var features = make([]Feature, 0, len(l.features))
SKIP:
	for i, f := range l.features {
		for _, j := range idxs {
			if i == j {
				continue SKIP
			}
		}
		features = append(features, f)
	}
}

func vectorTileValue(i interface{}) *vectorTile.Tile_Value {
	tv := new(vectorTile.Tile_Value)
	switch t := i.(type) {
	default:
		buff := new(bytes.Buffer)
		err := binary.Write(buff, binary.BigEndian, t)
		// We are going to ignore the value and return an empty TileValue
		if err == nil {
			tv.XXX_unrecognized = buff.Bytes()
		}

	case string:
		tv.StringValue = &t

	case fmt.Stringer:
		str := t.String()
		tv.StringValue = &str

	case bool:
		tv.BoolValue = &t

	case int8:
		intv := int64(t)
		tv.SintValue = &intv

	case int16:
		intv := int64(t)
		tv.SintValue = &intv

	case int32:
		intv := int64(t)
		tv.SintValue = &intv

	case int64:
		tv.IntValue = &t

	case uint8:
		intv := int64(t)
		tv.SintValue = &intv

	case uint16:
		intv := int64(t)
		tv.SintValue = &intv

	case uint32:
		intv := int64(t)
		tv.SintValue = &intv

	case uint64:
		tv.UintValue = &t

	case float32:
		tv.FloatValue = &t

	case float64:
		tv.DoubleValue = &t

	} // switch
	return tv
}
