package mvt

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

// Layer describes a layer in the tile. Each layer can have multiple features
// which describe drawing.
type Layer struct {
	// This is the name of the feature, is has to be unique within a tile.
	Name string
	// The set of features
	features []Feature
	extent   *int // default is 4096
}

func valMapToVTileValue(valMap []interface{}) (vt []*vectorTile.Tile_Value) {
	for _, v := range valMap {
		vt = append(vt, vectorTileValue(v))
	}
	return vt
}

// VTileLayer returns a vectorTile Tile_Layer object that represents this layer.
func (l *Layer) VTileLayer(ctx context.Context, extent tegola.BoundingBox) (*vectorTile.Tile_Layer, error) {
	kmap, vmap, err := keyvalMapsFromFeatures(l.features)
	if err != nil {
		return nil, err
	}
	valmap := valMapToVTileValue(vmap)
	var features = make([]*vectorTile.Tile_Feature, 0, len(l.features))
	for _, f := range l.features {
		if ctx.Err() != nil {
			return nil, context.Canceled
		}
		vtf, err := f.VTileFeature(ctx, kmap, vmap, extent, l.Extent())
		if err != nil {
			return nil, fmt.Errorf("Error getting VTileFeature: %v", err)
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

//Version is the version of tile spec this layer is from.
func (*Layer) Version() int {
	// Quick fix till we can get full version 2 compatibility.
	// TODO: gdey — look at issue #102 to get implementation to 2.1 spec.
	return 1
}

// Extent defaults to 4096
func (l *Layer) Extent() int {
	if l == nil || l.extent == nil {
		return 4096
	}
	return *(l.extent)
}

// SetExtent sets the extent value
func (l *Layer) SetExtent(e int) {
	if l == nil {
		l = new(Layer)
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

//AddFeatures will add one or more Features to the Layer, if a features ID is a the same as
//Any already in the Layer, it will ignore those features.
//If the id fields is nil, the feature will always be added.
func (l *Layer) AddFeatures(features ...Feature) (skipped bool) {

	b := make([]Feature, len(l.features), len(l.features)+len(features))
	copy(b, l.features)
	l.features = b
FEATURES_LOOP:
	for _, f := range features {
		if f.ID == nil {
			l.features = append(l.features, f)
			continue
		}
		for _, cf := range l.features {
			if cf.ID != nil {
				continue
			}
			// We matched, we skip
			if *cf.ID == *f.ID {
				skipped = true
				continue FEATURES_LOOP
			}
		}
		// There were no matches, let's add it to our list.
		l.features = append(l.features, f)
	}
	return skipped
}

//RemoveFeature allows you to remove one or more features, with the provided indexes.
//To figure out the indexes, use the indexs from the Features array.
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
