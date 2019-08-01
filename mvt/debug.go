package mvt

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/internal/convert"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
)

const debug = false

type geoDebugStruct struct {
	Min maths.Pt       `json:"min"`
	Max maths.Pt       `json:"max"`
	Geo basic.Geometry `json:"geo"`
}

func createDebugFile(min, max maths.Pt, geo tegola.Geometry, err error) {
	fln := os.Getenv("GenTestCase")
	if fln == "" {
		return
	}
	filename := fmt.Sprintf("/tmp/testcase_%v_%p.json", fln, geo)

	geo, err = basic.CloneGeometry(geo)
	if err != nil {
		log.Errorf("failed to clone geo for test case. %v", err)
		return
	}

	bgeo, err := convert.ToTegola(geo)
	if err != nil {
		log.Errorf("failed to convert geo for test case. %v", err)
		return
	}

	f, err := os.Create(filename)
	if err != nil {
		log.Errorf("failed to create test file %v : %v.", filename, err)
		return
	}
	defer f.Close()

	geodebug := geoDebugStruct{
		Max: max,
		Min: min,
		Geo: bgeo.(basic.Geometry),
	}

	if err = json.NewEncoder(f).Encode(geodebug); err != nil {
		log.Errorf("err encoding json: %v", err)
		return
	}

	log.Infof("created file: %v", filename)
}
