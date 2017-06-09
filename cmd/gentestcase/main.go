package main

import (
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/cmd/config"
	"github.com/terranodo/tegola/maths/validate"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/provider/postgis"
	"github.com/terranodo/tegola/wkb"
)

var cfg config.C
var pkgName = flag.String("package", "clip_test", "The package for the test case.")
var comment = flag.String("comment", "", "Additional comment to include at the start of file.")
var user string

func init() {
	cfg.InitFlags()
	flag.StringVar(&user, "user", os.Getenv("USER"), "The user who created the test case.")
}

func FormatBytes(bytes []byte) string {
	const (
		width      = 80 / 6 // 80 number of columns, 4 number of charcters
		byteFormat = "0x%0.2x, "
	)
	bytestr := []rune{}
	for i, b := range bytes {
		bytestr = append(bytestr, []rune(fmt.Sprintf(byteFormat, b))...)
		if (i+1)%width == 0 {
			bytestr[len(bytestr)-1] = '\n'
			bytestr = append(bytestr, '\t', '\t')
		}
	}
	return string(bytestr)
}

func GenerateTestCase(dir string) error {
	p, err := cfg.Provider()
	if err != nil {
		return err
	}

	tstcase := testcase{
		Command:     strings.Join(os.Args[1:], " "),
		User:        user,
		UserComment: *comment,
		PackageName: *pkgName,
		Date:        time.Now().Format("Mon Jan 2 2006 at 15:04:05"),
		X:           cfg.X(),
		Y:           cfg.Y(),
		Z:           cfg.Z(),
		LayerExtent: 4096,
		GeomVar:     "tgeom",
	}
	name, err := cfg.ProviderName()
	if err != nil {
		return err
	}

	err = p.ForEachFeatureBytes(
		name,
		cfg.Tile(),
		func(layer postgis.Layer, gid uint64, geom []byte, tags map[string]interface{}) error {

			if uint64(cfg.IsolateGeo) != gid {
				return nil
			}
			tc := tstcase

			h := sha1.New()
			h.Write(geom)
			tc.DataSha1 = hex.EncodeToString(h.Sum(nil))
			tc.Suffix = fmt.Sprintf("%v%v%v%v%v", cfg.Z(), cfg.X(), cfg.Y(), gid, tc.DataSha1)
			tc.Bytes = FormatBytes(geom)
			tc.LayerName = layer.Name
			tc.LayerSRID = layer.SRID
			tc.LayerSQL = layer.SQL
			tc.GID = gid

			src, err := gensrc(dataTmpl, &tc)
			output := strings.ToLower(fmt.Sprintf("testdata_%v_%v_test.go", gid, tc.DataSha1))
			if err = writeFile(dir, output, src); err != nil {
				return err
			}

			ggeom, err := wkb.DecodeBytes(geom)
			if err != nil {

				return err
			}
			tile := cfg.Tile()
			cursor := mvt.NewCursor(tile.BoundingBox(), tc.LayerExtent)
			var tgeo tegola.Geometry
			tgeo = ggeom
			if layer.SRID != tegola.WebMercator {
				// We need to convert our points to Webmercator.
				g, err := basic.ToWebMercator(layer.SRID, ggeom)
				if err != nil {

					return err
				}

				tgeo = g.Geometry
			}
			tgeo = cursor.ScaleGeo(tgeo)
			tc.Geom = tegola.GeometeryDecorator(tgeo, 10, "", nil)
			src, err = gensrc(geomTmpl, &tc)
			output = strings.ToLower(fmt.Sprintf("testgeom_%v_%v_test.go", gid, tc.DataSha1))
			if err = writeFile(dir, output, src); err != nil {
				return err
			}

			sg := mvt.SimplifyGeometry(tgeo, tile.ZEpislon())
			vg, err := validate.CleanGeometry(sg)
			if err != nil {
				return err
			}
			tc.Geom = tegola.GeometeryDecorator(vg, 10, "", nil)
			tc.GeomVar = "stgeom"
			src, err = gensrc(geomTmpl, &tc)
			output = strings.ToLower(fmt.Sprintf("testsgeom_%v_%v_test.go", gid, tc.DataSha1))
			if err = writeFile(dir, output, src); err != nil {
				return err
			}

			src, err = gensrc(generatedTmpl, &tc)
			if err != nil {
				return err
			}

			output = strings.ToLower(fmt.Sprintf("testcase_%v_%v_%v_%v_%v_test.go", cfg.Z(), cfg.X(), cfg.Y(), gid, tc.DataSha1))
			if err = writeFile(dir, output, src); err != nil {
				return err
			}

			return nil
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		path = "."
	}
	if path != "." {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Got following error attempting to created output dir (%v): %v\n", path, err)
			os.Exit(2)
		}
	}
	if cfg.IsolateGeo <= -1 {
		fmt.Fprintln(os.Stderr, "Geo id is requred.")
		os.Exit(3)
	}
	if err := GenerateTestCase(path); err != nil {
		fmt.Fprintf(os.Stderr, "Got the following error attempting to generate testcase: %v", err)
		os.Exit(2)
	}
}
