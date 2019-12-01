package atlas

import (
	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/mapbox/tilejson"
)

// GetTileJSON returns the base of a tilejson for the map (without tiles urls)
func (m Map) GetBaseTileJSON() tilejson.TileJSON {

	tileJSON := tilejson.TileJSON{
		Attribution: &m.Attribution,
		Bounds:      m.Bounds.Extent(),
		Center:      m.Center,
		Format:      "pbf",
		Name:        &m.Name,
		Scheme:      tilejson.SchemeXYZ,
		TileJSON:    tilejson.Version,
		Version:     "1.0.0",
		Grids:       make([]string, 0),
		Data:        make([]string, 0),
	}

	for i := range m.Layers {
		// check if the layer already exists in our slice. this can happen if the config
		// is using the "name" param for a layer to override the providerLayerName
		var skip bool
		for j := range tileJSON.VectorLayers {
			if tileJSON.VectorLayers[j].ID == m.Layers[i].MVTName() {
				// we need to use the min and max of all layers with this name
				if tileJSON.VectorLayers[j].MinZoom > m.Layers[i].MinZoom {
					tileJSON.VectorLayers[j].MinZoom = m.Layers[i].MinZoom
				}

				if tileJSON.VectorLayers[j].MaxZoom < m.Layers[i].MaxZoom {
					tileJSON.VectorLayers[j].MaxZoom = m.Layers[i].MaxZoom
				}

				skip = true
				break
			}
		}

		// the first layer sets the initial min / max otherwise they default to 0/0
		if len(tileJSON.VectorLayers) == 0 {
			tileJSON.MinZoom = m.Layers[i].MinZoom
			tileJSON.MaxZoom = m.Layers[i].MaxZoom
		}

		// check if we have a min zoom lower then our current min
		if tileJSON.MinZoom > m.Layers[i].MinZoom {
			tileJSON.MinZoom = m.Layers[i].MinZoom
		}

		// check if we have a max zoom higher then our current max
		if tileJSON.MaxZoom < m.Layers[i].MaxZoom {
			tileJSON.MaxZoom = m.Layers[i].MaxZoom
		}

		//	entry for layer already exists. move on
		if skip {
			continue
		}

		//	build our vector layer details
		layer := tilejson.VectorLayer{
			Version:     2,
			Extent:      4096,
			ID:          m.Layers[i].MVTName(),
			Name:        m.Layers[i].MVTName(),
			Description: m.Layers[i].MVTName(),
			MinZoom:     m.Layers[i].MinZoom,
			MaxZoom:     m.Layers[i].MaxZoom,
			Tiles:       []string{},
			Fields:      map[string]string{},
		}

		switch m.Layers[i].GeomType.(type) {
		case geom.Point, geom.MultiPoint:
			layer.GeometryType = tilejson.GeomTypePoint
		case geom.Line, geom.LineString, geom.MultiLineString:
			layer.GeometryType = tilejson.GeomTypeLine
		case geom.Polygon, geom.MultiPolygon:
			layer.GeometryType = tilejson.GeomTypePolygon
		default:
			layer.GeometryType = tilejson.GeomTypeUnknown
			// TODO: debug log
		}

		//Fields infos
		pLayers, err := m.Layers[i].Provider.Layers()
		if err == nil {
			for _, pl := range pLayers {
				if m.Layers[i].ProviderLayerName == pl.Name() && pl.IDFieldName() != "" {
					layer.Fields[pl.IDFieldName()] = "String"
				}
			}
		}

		// add our layer to our tile layer response
		tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)
	}
	return tileJSON
}
