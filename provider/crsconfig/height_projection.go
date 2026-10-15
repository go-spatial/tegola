package crsconfig

import (
	"fmt"
	"math"
	"sync"

	"github.com/alexeydott/proj/core"
	_ "github.com/alexeydott/proj/operations"
	"github.com/alexeydott/proj/support"
)

// HeightProjection owns a canonical WGS84 projection independent of the mutable
// projection registry. It transforms XY only; callers retain ellipsoidal Z.
type HeightProjection struct {
	mu         sync.Mutex
	definition string
	converter  core.IConvertLPToXY
	identity   bool
	datum      *core.System // Non-nil only for validated custom horizontal profiles.
}

// NewHeightProjection admits canonical WGS84 profiles, not custom definitions.
// Source admission must separately establish that this canonical CRS is its
// effective source definition; a numeric SRID is not evidence of height units.
func NewHeightProjection(srid uint64) (*HeightProjection, error) {
	var definition string
	switch {
	case srid == 4326:
		return &HeightProjection{identity: true, definition: "+proj=longlat +datum=WGS84"}, nil
	case srid == 3857:
		definition = "+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k=1.0"
	case srid >= 32601 && srid <= 32660:
		definition = fmt.Sprintf("+proj=utm +zone=%d +datum=WGS84 +units=m +no_defs", srid-32600)
	case srid >= 32701 && srid <= 32760:
		definition = fmt.Sprintf("+proj=utm +zone=%d +south +datum=WGS84 +units=m +no_defs", srid-32700)
	default:
		return nil, fmt.Errorf("height-preserving horizontal CRS %d is unsupported", srid)
	}
	parsed, err := support.NewProjString(definition)
	if err != nil {
		return nil, fmt.Errorf("canonical height projection: %w", err)
	}
	_, operation, err := core.NewSystem(parsed)
	if err != nil {
		return nil, fmt.Errorf("canonical height projection: %w", err)
	}
	converter, ok := operation.(core.IConvertLPToXY)
	if !ok {
		return nil, fmt.Errorf("canonical height projection has no XY converter")
	}
	return &HeightProjection{definition: definition, converter: converter}, nil
}

// Definition reports the immutable canonical projection used by this instance.
func (p *HeightProjection) Definition() string {
	if p == nil {
		return ""
	}
	return p.definition
}

// Forward transforms WGS84 longitude/latitude degrees to canonical source XY.
func (p *HeightProjection) Forward(input []float64) ([]float64, error) {
	return p.transform(input, false)
}

// Inverse transforms canonical source XY to WGS84 longitude/latitude degrees.
func (p *HeightProjection) Inverse(input []float64) ([]float64, error) {
	return p.transform(input, true)
}

func (p *HeightProjection) transform(input []float64, inverse bool) ([]float64, error) {
	if p == nil {
		return nil, fmt.Errorf("height projection is nil")
	}
	if !p.identity && p.converter == nil {
		return nil, fmt.Errorf("height projection is uninitialized")
	}
	if len(input)%2 != 0 {
		return nil, fmt.Errorf("height projection requires coordinate pairs")
	}
	for _, coordinate := range input {
		if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
			return nil, fmt.Errorf("height projection requires finite coordinates")
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	output := make([]float64, len(input))
	for i := 0; i < len(input); i += 2 {
		if p.identity {
			output[i], output[i+1] = input[i], input[i+1]
		} else if inverse {
			lp, err := p.converter.Inverse(&core.CoordXY{X: input[i], Y: input[i+1]})
			if err != nil {
				return nil, fmt.Errorf("height projection inverse: %w", err)
			}
			transformDatum(p.datum, lp, false)
			output[i], output[i+1] = support.RToDD(lp.Lam), support.RToDD(lp.Phi)
		} else {
			if math.Abs(input[i+1]) > 90 {
				return nil, fmt.Errorf("projection latitude outside valid domain")
			}
			lp := &core.CoordLP{Lam: support.DDToR(input[i]), Phi: support.DDToR(input[i+1])}
			transformDatum(p.datum, lp, true)
			xy, err := p.converter.Forward(lp)
			if err != nil {
				return nil, fmt.Errorf("height projection forward: %w", err)
			}
			output[i], output[i+1] = xy.X, xy.Y
		}
		if math.IsNaN(output[i]) || math.IsNaN(output[i+1]) || math.IsInf(output[i], 0) || math.IsInf(output[i+1], 0) {
			return nil, fmt.Errorf("height projection produced nonfinite coordinates")
		}
	}
	return output, nil
}
