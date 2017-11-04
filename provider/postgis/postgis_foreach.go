package postgis

import (
	"context"
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/wkb"
)

func (p *Provider) Layer(name string) (Layer, bool) {
	if name == "" {
		return p.layers[p.firstlayer], true
	}
	plyr, ok := p.layers[name]
	return plyr, ok
}

func (p *Provider) ForEachFeature(ctx context.Context, layerName string, tile tegola.Tile, fn func(layer Layer, gid uint64, geom wkb.Geometry, tags map[string]interface{}) error) error {
	plyr, ok := p.Layer(layerName)
	if !ok {
		return fmt.Errorf("Don't know of the layer named “%v”", layerName)
	}

	sql, err := replaceTokens(&plyr, tile)
	if err != nil {
		return fmt.Errorf("Got the following error (%v) running this sql (%v)", err, sql)
	}

	// do a quick context check:
	if ctx.Err() != nil {
		return mvt.ErrCanceled
	}

	rows, err := p.pool.Query(sql)
	if err != nil {
		return fmt.Errorf("Got the following error (%v) running this sql (%v)", err, sql)
	}
	defer rows.Close()

	//	fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	fdescs := rows.FieldDescriptions()

	for rows.Next() {
		// do a quick context check:
		if ctx.Err() != nil {
			return mvt.ErrCanceled
		}

		//	fetch row values
		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error running SQL: %v ; %v", sql, err)
		}

		gid, geobytes, tags, err := decipherFields(ctx, plyr.GeomFieldName(), plyr.IDFieldName(), fdescs, vals)
		if err != nil {
			return fmt.Errorf("For layer(%v) %v", plyr.Name(), err)
		}

		//	decode our WKB
		geom, err := wkb.DecodeBytes(geobytes)
		if err != nil {
			return fmt.Errorf("Unable to decode geometry field (%v) into wkb where (%v = %v).", plyr.GeomFieldName(), plyr.IDFieldName(), gid)
		}
		if err = fn(plyr, gid, geom, tags); err != nil {
			return err
		}
	}
	return nil
}
