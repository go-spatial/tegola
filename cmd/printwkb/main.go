package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/cmd/printwkb/utils"
	"github.com/terranodo/tegola/config"
)

var configFile string
var mapName string
var providerLayer string
var coords [3]int
var isolateGeo int

func init() {
	const (
		defaultConfigFile  = "config.toml"
		usageConfigFile    = "The config file for tegola."
		usageMapName       = "The map name to use. If one isn't provided the first map is used."
		usageProviderLayer = "The Provider and the Layer to use — must be a postgis provider. “$provider.$layer” [required]"
	)
	flag.StringVar(&configFile, "config", defaultConfigFile, usageConfigFile)
	flag.StringVar(&configFile, "c", defaultConfigFile, usageConfigFile+" (shorthand)")
	flag.StringVar(&providerLayer, "provider", "", usageProviderLayer)
	flag.StringVar(&providerLayer, "p", "", usageProviderLayer+" (shorthand)")
	flag.IntVar(&(coords[0]), "z", 0, "The Z coord")
	flag.IntVar(&(coords[1]), "x", 0, "The X coord")
	flag.IntVar(&(coords[2]), "y", 0, "The Y coord")
	flag.IntVar(&isolateGeo, "g", -1, "Only grab the geo described. -1 means all of them.")
}

func splitProviderLayer(providerLayer string) (provider, layer string) {
	parts := strings.SplitN(providerLayer, ".", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}

type ProviderLayer struct {
	srid     int
	database string
	port     uint16
	user     string
	password string
	host     string
	geoField string
	geoID    string
	table    string
}

func (pl ProviderLayer) pgxConfig() pgx.ConnPoolConfig {
	return pgx.ConnPoolConfig{
		ConnConfig: pgx.ConnConfig{
			Host:     pl.host,
			Port:     pl.port,
			Database: pl.database,
			User:     pl.user,
			Password: pl.password,
		},
	}
}

func LoadProvider(configFile string, providerlayer string) (pl ProviderLayer, err error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		return pl, err
	}
	if len(cfg.Providers) == 0 {
		return pl, fmt.Errorf("No Providers defined in config.")
	}
	providerName, layerName := splitProviderLayer(providerLayer)
	_ = layerName
	var providerLayer map[string]interface{}
	var provider map[string]interface{}
	if providerName == "" {
		// Need to look up the provider
		for _, p := range cfg.Providers {
			t, _ := p["type"].(string)
			if t != "postgis" {
				continue
			}
			provider = p
			providerName, _ = p["name"].(string)
			break
		}
	} else {
		for _, p := range cfg.Providers {
			t, _ := p["type"].(string)
			if t != "postgis" {
				continue
			}
			name, _ := p["name"].(string)
			if name != providerName {
				continue
			}
			provider = p
			break
		}
		if provider == nil {
			return pl, fmt.Errorf("Cound not find provider(%v).", providerName)
		}
	}
	var ok bool

	if _, ok = provider["srid"]; ok {
		srid, ok := provider["srid"].(int64)
		if !ok {
			return pl, fmt.Errorf("Cound not convert %T", provider["srid"])
		}
		pl.srid = int(srid)
	} else {
		pl.srid = tegola.WebMercator
	}

	port, ok := provider["port"].(int64)
	if !ok {
		return pl, fmt.Errorf("Cound not convert %T", provider["port"])
	}
	pl.port = uint16(port)
	if pl.database, ok = provider["database"].(string); !ok {
		return pl, fmt.Errorf("Cound not convert %T", provider["database"])
	}
	if pl.user, ok = provider["user"].(string); !ok {
		return pl, fmt.Errorf("Cound not convert %T", provider["user"])
	}
	if pl.host, ok = provider["host"].(string); !ok {
		return pl, fmt.Errorf("Cound not convert %T", provider["host"])
	}
	if pl.password, ok = provider["password"].(string); !ok {
		return pl, fmt.Errorf("Cound not convert %T", provider["password"])
	}
	layers, ok := provider["layers"].([]map[string]interface{})
	if !ok {
		return pl, fmt.Errorf("Cound not convert %T", provider["layers"])
	}
	if layerName == "" {
		providerLayer = layers[0]
		layerName, _ = providerLayer["name"].(string)
	} else {
		for _, lyr := range layers {
			ln, _ := lyr["name"].(string)
			if ln == layerName {
				providerLayer = lyr
				break
			}
		}
	}
	if pl.geoField, ok = providerLayer["geometry_fieldname"].(string); !ok {
		return pl, fmt.Errorf("was not able to convert geometry_fieldname to string %v.", providerLayer["geometry_fieldname"])
	}
	if pl.geoID, ok = providerLayer["id_fieldname"].(string); !ok || pl.geoID == "" {
		pl.geoID = "gid"
	}
	if pl.table, ok = providerLayer["tablename"].(string); !ok || pl.table == "" {
		pl.table = layerName
	}
	return pl, nil
}

func main() {

	flag.Parse()
	provider, err := LoadProvider(configFile, providerLayer)
	if err != nil {
		panic(err)
	}

	tile := tegola.Tile{
		X: coords[1],
		Y: coords[2],
		Z: coords[0],
	}
	bbox := tile.BoundingBox()
	minGeo, err := basic.FromWebMercator(provider.srid, basic.Point{bbox.Minx, bbox.Miny})
	if err != nil {
		panic(err)
	}
	maxGeo, err := basic.FromWebMercator(provider.srid, basic.Point{bbox.Maxx, bbox.Maxy})
	if err != nil {
		panic(err)
	}
	minPt := minGeo.AsPoint()
	maxPt := maxGeo.AsPoint()

	pool, err := pgx.NewConnPool(provider.pgxConfig())
	if err != nil {
		panic(fmt.Sprintf("Failed while creating connection pool: %v", err))
	}
	sql := fmt.Sprintf(
		`SELECT  ST_AsBinary("%v") AS "geometry" from "%v" WHERE "%[1]v" && ST_MakeEnvelope(%[3]v,%v,%v,%v,%v)`,
		provider.geoField,
		provider.table,
		minPt.X(),
		minPt.Y(),
		maxPt.X(),
		maxPt.Y(),
		provider.srid,
	)
	fmt.Println("// SQL:", sql)
	rows, err := pool.Query(sql)
	if err != nil {
		panic(fmt.Sprintf("Got the following error (%v) running this sql (%v)", err, sql))
	}
	defer rows.Close()
	//	fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	var geobytes []byte
	var ok bool
	var count = -1
	for rows.Next() {
		count++
		if isolateGeo != -1 && isolateGeo != count {
			log.Printf("Skipping %v", count)
			continue
		}
		vals, err := rows.Values()
		if err != nil {
			panic(fmt.Sprintf("Got an error trying to run SQL: %v ; %v", sql, err))
		}
		if geobytes, ok = vals[0].([]byte); !ok {
			panic("Was unable to convert geometry field into bytes.")
		}
		utils.PrintWkbDesc(fmt.Sprintf("wkb%v", count), provider.srid, coords[0], coords[1], coords[2], geobytes)
	}
	fmt.Println("// Number of geometries found:", count+1)
}
