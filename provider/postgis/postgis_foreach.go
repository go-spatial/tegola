package postgis

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/terranodo/tegola"
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
		return fmt.Errorf("layer (%v) not found ", layerName)
	}

	sql, err := replaceTokens(&plyr, tile)
	if err != nil {
		return fmt.Errorf("error running layer (%v) SQL (%v): %v", layerName, sql, err)
	}

	if strings.Contains(os.Getenv("SQL_DEBUG"), "EXECUTE_SQL") {
		log.Printf("SQL_DEBUG:EXECUTE_SQL for layer (%v): %v", layerName, sql)
	}

	// do a quick context check:
	if err := ctx.Err(); err != nil {
		return err
	}

	rows, err := p.pool.Query(sql)
	if err != nil {
		return fmt.Errorf("error running layer (%v) SQL (%v): %v", layerName, sql, err)
	}
	defer rows.Close()

	//	fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	fdescs := rows.FieldDescriptions()

	for rows.Next() {
		// do a quick context check:
		if err := ctx.Err(); err != nil {
			return err
		}

		//	fetch row values
		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error running layer (%v) SQL (%v): %v", layerName, sql, err)
		}

		gid, geobytes, tags, err := decipherFields(ctx, plyr.GeomFieldName(), plyr.IDFieldName(), fdescs, vals)
		if err != nil {
			switch err {
			case context.Canceled:
				return err
			default:
				return fmt.Errorf("For layer (%v) %v", plyr.Name(), err)
			}
		}

		//	decode our WKB
		geom, err := wkb.DecodeBytes(geobytes)
		if err != nil {
			return fmt.Errorf("unable to decode layer (%v) geometry field (%v) into wkb where (%v = %v). err: %v", layerName, plyr.GeomFieldName(), plyr.IDFieldName(), gid, err)
		}
		if err = fn(plyr, gid, geom, tags); err != nil {
			return err
		}
	}
	return nil
}
