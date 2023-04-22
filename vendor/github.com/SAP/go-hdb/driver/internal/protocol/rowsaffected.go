package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// rows affected
const (
	raSuccessNoInfo   = -2
	RaExecutionFailed = -3
)

// RowsAffected represents a rows affected part.
type RowsAffected []int32

func (r RowsAffected) String() string {
	return fmt.Sprintf("%v", []int32(r))
}

func (r *RowsAffected) reset(numArg int) {
	if r == nil || numArg > cap(*r) {
		*r = make(RowsAffected, numArg)
	} else {
		*r = (*r)[:numArg]
	}
}

func (r *RowsAffected) decode(dec *encoding.Decoder, ph *PartHeader) error {
	r.reset(ph.numArg())

	for i := 0; i < ph.numArg(); i++ {
		(*r)[i] = dec.Int32()
	}
	return dec.Error()
}

// Total return the total number of all affected rows.
func (r RowsAffected) Total() int64 {
	if r == nil {
		return 0
	}

	total := int64(0)
	for _, rows := range r {
		if rows > 0 {
			total += int64(rows)
		}
	}
	return total
}
