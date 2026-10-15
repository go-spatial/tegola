package auth

import (
	"fmt"
	"math"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// Prms represents authentication parameters.
type Prms struct {
	prms []any
}

// AddCESU8String adds a CESU8 string parameter.
func (p *Prms) AddCESU8String(s string) { p.prms = append(p.prms, s) } // unicode string
func (p *Prms) addEmpty()               { p.prms = append(p.prms, []byte{}) }
func (p *Prms) addBytes(b []byte)       { p.prms = append(p.prms, b) }

// AddString adds a string parameter encoded as raw bytes to distinguish it
// from a unicode string parameter.
func (p *Prms) AddString(s string) { p.prms = append(p.prms, []byte(s)) }
func (p *Prms) addPrms() *Prms {
	prms := &Prms{}
	p.prms = append(p.prms, prms)
	return prms
}

// Encode encodes the parameters.
func (p *Prms) Encode(enc *encoding.Encoder) error {
	numPrms := len(p.prms)
	if numPrms > math.MaxInt16 {
		return fmt.Errorf("invalid number of parameters %d - maximum %d", numPrms, math.MaxInt16)
	}
	enc.Int16(int16(numPrms))

	for _, e := range p.prms {
		switch e := e.(type) {
		case []byte:
			if err := enc.LIBytes(e); err != nil {
				return err
			}
		case string:
			if err := enc.CESU8LIString(e); err != nil {
				return err
			}
		case *Prms:
			subEnc := encoding.NewEncoder(make([]byte, 0), enc.Transformer())
			if err := e.Encode(subEnc); err != nil {
				return err
			}
			if err := enc.LIBytes(subEnc.Buffer()); err != nil {
				return err
			}
		default:
			panic("invalid parameter") // should not happen
		}
	}
	return nil
}
