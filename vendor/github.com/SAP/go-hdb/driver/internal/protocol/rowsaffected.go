package protocol

import (
	"fmt"
	"slices"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// rows affected.
const (
	raTBD             = -1
	raSuccessNoInfo   = -2
	raExecutionFailed = -3
)

// RowsAffected represents a rows affected part.
type RowsAffected struct {
	rows []int32
}

func (r RowsAffected) String() string {
	return fmt.Sprintf("%v", r.rows)
}

func (r *RowsAffected) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	numArg := header.numArg()
	r.rows = slices.Grow(r.rows, numArg)[:numArg]

	for i := range numArg {
		r.rows[i] = dec.Int32()
	}
	return nil
}

// Total returns the total number of all affected rows.
func (r RowsAffected) Total() int64 {
	total := int64(0)
	for _, rows := range r.rows {
		if rows > 0 { // add only positive number / negatives are status / error values (see above)
			total += int64(rows)
		}
	}
	return total
}

// SetHDbErrorsStmtNo sets the HDBErrors statement numbers relatively to an offset.
func (r RowsAffected) SetHDbErrorsStmtNo(errs *HdbErrors, offset int) {
	if errs == nil {
		return
	}
	j := 0
	for i, rows := range r.rows {
		if rows == raExecutionFailed {
			errs.setStmtNo(j, offset+i)
			j++
		}
	}
}
