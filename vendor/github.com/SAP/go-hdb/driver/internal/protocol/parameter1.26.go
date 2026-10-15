//go:build !go1.27

package protocol

import (
	"fmt"
	"math/bits"
	"slices"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

func (f *ParameterField) decodeResult(dec *encoding.Decoder, attrs *ReaderAttrs, lobReader LobReader) (any, error) {
	return decodeResult(f.tc, dec, attrs, lobReader, f.scale)
}

func (p *OutputParameters) decodeResult(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs, lobReader LobReader) error {
	numArg := header.numArg()

	cols := len(p.OutputFields)
	if numArg < 0 {
		return fmt.Errorf("invalid number of arguments %d", numArg)
	}
	if hi, _ := bits.Mul(uint(numArg), uint(cols)); hi != 0 {
		return fmt.Errorf("result set too large: %d rows x %d cols", numArg, cols)
	}

	n := numArg * cols
	p.FieldValues = slices.Grow(p.FieldValues, n)[:n]

	for i := range numArg {
		for j, f := range p.OutputFields {
			var err error
			if p.FieldValues[i*cols+j], err = f.decodeResult(dec, attrs, lobReader); err != nil {
				p.DecodeErrors = append(p.DecodeErrors, &DecodeError{row: i, fieldName: f.Name(), err: err}) // collect decode / conversion errors
			}
		}
	}
	return nil
}
