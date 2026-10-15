//go:build go1.27

package protocol

import (
	"fmt"
	"math/bits"
	"slices"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

func (f *ResultField) decodeResult(dec *encoding.Decoder, attrs *ReaderAttrs) (any, error) {
	return decodeResult(f.tc, dec, attrs, f.scale)
}

func (r *Resultset) decodeResult(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	numArg := header.numArg()

	cols := len(r.ResultFields)
	if numArg < 0 {
		return fmt.Errorf("invalid number of arguments %d", numArg)
	}
	if hi, _ := bits.Mul(uint(numArg), uint(cols)); hi != 0 {
		return fmt.Errorf("result set too large: %d rows x %d cols", numArg, cols)
	}
	n := numArg * cols
	r.FieldValues = slices.Grow(r.FieldValues, n)[:n]

	for i := range numArg {
		for j, f := range r.ResultFields {
			var err error
			if r.FieldValues[i*cols+j], err = f.decodeResult(dec, attrs); err != nil {
				r.DecodeErrors = append(r.DecodeErrors, &DecodeError{row: i, fieldName: f.Name(), err: err}) // collect decode / conversion errors
			}
		}
	}
	return nil
}
