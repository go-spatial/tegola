package driver

import (
	_ "embed" // embed stats configuration
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"
)

//go:embed statscfg.json
var statsCfgRaw []byte

var statsCfg struct {
	SQLTimeTexts    []string  `json:"sqlTimeTexts"`
	TimeUpperBounds []float64 `json:"timeUpperBounds"`
}

func loadStatsCfg() error {

	if err := json.Unmarshal(statsCfgRaw, &statsCfg); err != nil {
		return fmt.Errorf("invalid statscfg.json file: %s", err)
	}

	if len(statsCfg.SQLTimeTexts) != int(numSQLTime) {
		return fmt.Errorf("invalid number of statscfg.json sqlTimeTexts %d - expected %d", len(statsCfg.SQLTimeTexts), numSQLTime)
	}
	if len(statsCfg.TimeUpperBounds) == 0 {
		return fmt.Errorf("number of statscfg.json timeUpperBounds needs to be greater than %d", 0)
	}

	// sort and dedup timeBuckets
	slices.Sort(statsCfg.TimeUpperBounds)
	statsCfg.TimeUpperBounds = slices.Compact(statsCfg.TimeUpperBounds)

	return nil
}
