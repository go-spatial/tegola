package postgis

import (
	"text/template"

	"github.com/jackc/pgx"

	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
)

// Query holds information about a query.
type Query struct {
	// The SQL to use. !BBOX! token will be replaced by the envelope
	SQL *template.Template
	// The ID field name, this will default to 'gid' if not set to something other then empty string.
	IDFieldname string
	// The Geometery field name, this will default to 'geom' if not set to soemthing other then empty string.
	GeomFieldName string
}

// Provider provides the postgis data provider.
type Provider struct {
	config pgx.ConnPoolConfig
	pool   *pgx.ConnPool
	layers map[string]Query // map of layer name and corrosponding sql
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
	// Each layer should have a name a query that describes how to get the Features for that layer.
	Layers map[string]Query
	SRID   *int // Defaults to 3857
}

// DEFAULT sql for get geometeries,
const stdSQL = `
SELECT *
FROM
	%[1]v
WHERE
	%[2]v && {{.BBox}}
`

const Name = "postgis"

func init() {
	provider.Register(Name, NewProvider)
}

type confType map[string]interface{}

func (c confType) getString(key string)(v string,err error){

	var val interface{}
	var ok bool
	if val, ok =  config[key]; !ok {
		return v, fmt.Errorf("%v key missing in config for %v provider.",key,Name)
	}
	if v, ok = val.(string); !ok {
		return v, fmt.Errorf("%v value needs to be a string.",key)
	}
	return v, nil
}
func (c confType) getuInt16(key string)(v uint16,err error){
	var val interface{}
	var ok bool
	if val, ok =  config[key]; !ok {
		return v, fmt.Errorf("%v key missing in config for %v provider.",key,Name)
	}
	if v, ok = val.(uint16); !ok {
		return v, fmt.Errorf("%v value needs to be an uint16 value.",key)
	}
	return v, nil
}
func (c confType) getPInt8(key string,def int8)(v int8,err error){
	var val interface{}
	var ok bool
	if val, ok =  config[key]; !ok {
		return v, fmt.Errorf("%v key missing in config for %v provider.",key,Name)
	}
	if v, ok = val.(*int8); !ok {
		return v, fmt.Errorf("%v value needs to be an uint16 value.",key)
	}
	if v == nil {
		return def, nil
	}
	return *v, nil
}

// NewProvider Setups and returns a new postgis provide or an error; if something
// is wrong. The function will validate that the config object looks good before
// trying to create a driver. This means that the Provider expects the following
// fields to exists in the provided map[string]interface{} map.
// Host string — the host to connect to.
// Port uint16 — the port to connect on.
// Database string — the database name
// User string — the user name
// Password string — the Password
// MaxConnections *uint8 // Default is 5 if nil, 0 means no max.
// Queries map[string]struct{ — This is map of layers keyed by the layer name.
//     TableName string || SQL string — This is the sql to use or the tablename to use with the default query.
//     Fields []string — This is a list, if this is nil or empty we will get all fields.
//     GeometryFieldname string — This is the field name of the geometry, if it's an empty string or nil, it will defaults to 'geom'.
//     IDFieldname string — This is the field name for the id property, if it's an empty string or nil, it will defaults to 'gid'.
//  }
func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	// Validate the config to make sure it has the values I care about and the types for those values.
	c := confType(config)
	host, err := c.getString("host")
	if err != nil {
		return nil, err
	}
	port, err := c.getuInt16("port")
	if err != nil {
		return nil, err
	}
	db, err := c.getString("db")
	if err != nil {
		return nil, err
	}
	user, err := c.getString("user")
	if err != nil {
		return nil, err
	}
	password, err := c.getString("password")
	if err != nil {
		return nil, err
	}
	mcon, err := c.




}

// NewProviderOld Setups and returns a new postgis provider that can be used to get
// tiles for layers.
// name is the name for this Provider.
// layers is a map of the layer name to the sql to run on postgis
/*
func NewProviderOld(config Config) (*Provider, error) {

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

	for name, query := range config.Layers {
		tpl := template.New(name)
		if query.SQL == ""
		//	check for template
		if !strings.Contains(tplStr, "!BBOX!") {
			tplStr = fmt.Sprintf(stdSQL, tplStr, "geom", "gid", ",name")
		}
		if _, err := tpl.Parse(tplStr); err != nil {
			return nil, fmt.Errorf("Layer %v template( %v ) had an error: %v", name, tplStr, err)
		}
		p.layers[name] = tpl
	}
	return &p, nil
}
*/
// MVTLayer returns a mvt.Layer
/*
func (p *Provider) MVTLayer(layerName string, tile tegola.Tile, tags map[string]interface{}) (layer *mvt.Layer, err error) {
	if p == nil {
		return nil, fmt.Errorf("Provider is nil")
	}

	extent := tile.Extent()

	//	build out our template bbox template
	tpl := struct {
		Name string
		BBox string
	}{
		Name: layerName,
		BBox: fmt.Sprintf("ST_MakeEnvelope(%v,%v,%v,%v,%v)", extent.Minx, extent.Miny, extent.Maxx, extent.Maxy, p.srid),
	}

	var sr bytes.Buffer

	t, ok := p.layers[layerName]
	if !ok {
		return nil, fmt.Errorf("Don't know of the layer %v", layerName)
	}
	//	execute our template
	t.Execute(&sr, tpl)

	sql := sr.String()

	//	log.Println("sql", sql)

	//	execute query
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
		//	tags
			gtags := map[string]interface{}{
				"class": "park",
			}

		//	scan data returned from database
		if err = rows.Scan(&rgeom, &gname, &gid); err != nil {
			return nil, err
		}

		//	gecode our geometry
		geom, err := wkb.DecodeBytes(rgeom)
		if err != nil {
			return nil, err
		}
		//		log.Printf("Got geo: %v", wkb.WKT(geom))

		//	TODO: Need to support collection geometries.
		if _, ok := geom.(tegola.Collection); ok {
			return nil, fmt.Errorf("For Layer (%v) and geometry name(%v); Geometry collections are not supported.", layerName, gname)
		}

		if err != nil {
			geostr := wkb.WKT(geom)
			return nil, fmt.Errorf("Error trying to rehome %v : %v", geostr, err)
		}

		//	add features to layer
		layer.AddFeatures(mvt.Feature{
			ID:       &gid,
			Tags:     gtags,
			Geometry: geom,
		})
	}

	return layer, nil
}
*/
