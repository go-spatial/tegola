package mvt

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
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
	bgeo, err := basic.CloneGeometry(geo)
	if err != nil {
		log.Errorf("failed to clone geo for test case. %v", err)
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
		Geo: bgeo,
	}
	enc := json.NewEncoder(f)
	enc.Encode(geodebug)
	log.Infof("created file: %v", filename)
}
