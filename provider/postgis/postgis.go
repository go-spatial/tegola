package postgis

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/jackc/pgx"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/wkb"
)

//Provider provides the postgis data provider.
type Provider struct {
	config pgx.ConnPoolConfig
	pool   *pgx.ConnPool
	layers map[string]*template.Template // map of layer name and corrosponding sql
	srid   int
}

// Config is the main config structure for configuring this Provider.
type Config struct {
	Host           string
	Port           uint16
	Database       string
	User           string
	Password       string
	MaxConnections *uint8 // Default is 5 if nil, 0 means no max.
	Layers         map[string]string
	SRID           *int // Defaults to 3857
}

// NewProvider Setups and returns a new postgis provider that can be used to get
// tiles for layers.
// name is the name for this Provider.
// layers is a map of the layer name to the sql to run on postgis
func NewProvider(config Config) (*Provider, error) {

	conf := pgx.ConnConfig{
		Host:     config.Host,
		Port:     config.Port,
		Database: config.Database,
		User:     config.User,
		Password: config.Password,
	}
	srid := 3857
	if config.SRID != nil {
		srid = *config.SRID
	}
	mconn := 5
	if config.MaxConnections != nil {
		mconn = int(*config.MaxConnections)
	}
	poolConfig := pgx.ConnPoolConfig{
		MaxConnections: mconn,
		ConnConfig:     conf,
	}
	connPool, err := pgx.NewConnPool(poolConfig)
	if err != nil {
		return nil, err
	}

	p := Provider{
		config: poolConfig,
		pool:   connPool,
		srid:   srid,
		layers: map[string]*template.Template{},
	}
	for name, tplStr := range config.Layers {
		tpl := template.New(name)
		if !(strings.Contains(tplStr, "{{") && strings.Contains(tplStr, "}}")) {
			tplStr = fmt.Sprintf(`SELECT ST_AsBinary(ST_TRANSFORM("geom",%[2]v)) AS geom,"name","gid" FROM %[1]v WHERE "geom" && ST_TRANSFORM({{.BBox}}, 4326) LIMIT 5`, tplStr, p.srid)
		}
		_, err := tpl.Parse(tplStr)
		if err != nil {
			return nil, fmt.Errorf("Layer %v template( %v ) had an error: %v", name, tplStr, err)
		}
		p.layers[name] = tpl
	}
	return &p, nil
}

//MVTLayer returns a mvt.Layer
func (p *Provider) MVTLayer(layerName string, tile tegola.Tile) (layer *mvt.Layer, err error) {
	if p == nil {
		return nil, fmt.Errorf("Provider is nil")
	}
	ulx, uly, llx, lly := tile.BBox()
	tpl := struct {
		Name string
		BBox string
	}{
		Name: layerName,
		BBox: fmt.Sprintf("ST_MakeEnvelope(%v,%v,%v,%v,%v)", ulx, uly, llx, lly, p.srid),
	}
	var sr bytes.Buffer

	t, ok := p.layers[layerName]
	if !ok {
		return nil, fmt.Errorf("Don't know of the layer %v", layerName)
	}
	t.Execute(&sr, tpl)
	sql := sr.String()
	log.Printf("Running sql:\n%v\n", sql)
	rows, err := p.pool.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	layer = new(mvt.Layer)
	layer.Name = layerName
	// Iterate through the result set
	for rows.Next() {
		var rgeom []byte
		gname := new(*string)
		var gid uint64
		gtags := map[string]interface{}{
			"class": "park",
			"type":  "city",
		}
		err = rows.Scan(&rgeom, &gname, &gid)
		if err != nil {
			return nil, err
		}
		geom, err := wkb.DecodeBytes(rgeom)
		if err != nil {
			return nil, err
		}
		/*
			if gname != nil {
				gtags["name"] = *gname
			}
		*/
		//TODO: Need to support collection geometries.
		if _, ok := geom.(tegola.Collection); ok {
			return nil, fmt.Errorf("For Layer (%v) and geometry name(%v); Geometry collections are not supported.", layerName, gname)
		}
		// rehgeom, err := basic.RehomeGeometry(geom, ulx, uly)
		if err != nil {
			geostr := wkb.WKT(geom)
			return nil, fmt.Errorf("Error trying to rehome %v : %v", geostr, err)
		}
		layer.AddFeatures(mvt.Feature{
			ID:       &gid,
			Tags:     gtags,
			Geometry: geom,
		})
	}
	log.Printf("Layer looks like %+v\n", layer)
	return layer, nil
}
