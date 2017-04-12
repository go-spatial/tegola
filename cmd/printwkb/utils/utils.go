package utils

import (
	"fmt"
	"os"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/wkb"
)

type WKBDesc struct {
	IsBigEndian bool
	Coords      []int
	SRID        int
	Bytes       []byte
}

func FormatBytes(name string, srid int, z, x, y int, bytes []byte) string {
	const (
		width      = 80 / 6 // 80 number of columns, 4 number of charcters
		sridLine   = "SRID: %v,"
		coordsLine = "Coords:[]int{%v,%v,%v},"
		format     = "var %v = utils.WKBDesc{\n\t" +
			"\n\t" + sridLine +
			"\n\t" + coordsLine +
			"\n\tBytes:[]byte{\n\t\t%v\n\t},\n}\n"
		byteFormat = "0x%0.2x, "
	)
	vname := "e"
	if name != "" {
		vname = name
	}

	bytestr := []rune{}
	for i, b := range bytes {
		bytestr = append(bytestr, []rune(fmt.Sprintf(byteFormat, b))...)
		if (i+1)%width == 0 {
			bytestr[len(bytestr)-1] = '\n'
			bytestr = append(bytestr, '\t', '\t')
		}
	}
	return fmt.Sprintf(format, vname, srid, z, x, y, string(bytestr))
}

func Print(srid int, geobytes []byte, x, y, z int) {
	tile := tegola.Tile{
		X: x,
		Y: y,
		Z: z,
	}

	fmt.Println("/*\n")
	defer fmt.Println("\n*/")
	var geo tegola.Geometry
	var err error
	geo, err = wkb.DecodeBytes(geobytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding goemetry: %v", err)
		os.Exit(2)
	}
	if geo == nil {
		fmt.Fprintf(os.Stderr, "Geo is nil.")
		os.Exit(0)
	}
	if srid != tegola.WebMercator {
		fmt.Println("Did convert geo to Webmercator.")
		g, err := basic.ToWebMercator(srid, geo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to convert to WebMercator %v.", err)
			os.Exit(0)
		}
		geo = g.Geometry
	}

	fmt.Printf("RAW GEO:\n%#v\n", basic.Clone(geo))

	c := mvt.NewCursor(tile.BoundingBox(), 4096)
	g := c.ScaleGeo(geo)
	cmin, cmax := c.MinMax()
	// Region{Min: maths.Pt{X: 0, Y: 0}, Max: maths.Pt{X: 10, Y: 10}, Extant: 1},

	fmt.Println("//func main(){")
	fmt.Printf("// rec := []float64{%v,%v,%v,%v}\n", cmin.X, cmin.Y, cmax.X, cmax.Y)
	fmt.Printf("// tile := tegola.Tile{Z:%v, X:%v,Y:%v}\n", z, x, y)
	fmt.Println("// c := mvt.NewCursor(tile.BoundingBox(),4096)")
	fmt.Println("//}")
	fmt.Printf("// Scaled GEO:\n%#v\n", g)
	cg, err := c.ClipGeo(g)
	if err != nil {
		panic(err)
	}
	fmt.Printf("// Clip GEO:\n%#v\n", cg)
}

func PrintWkbDesc(name string, srid int, z, x, y int, wkbbytes []byte) {
	fmt.Println(FormatBytes(name, srid, z, x, y, wkbbytes))

	Print(srid, wkbbytes, x, y, z)
}
