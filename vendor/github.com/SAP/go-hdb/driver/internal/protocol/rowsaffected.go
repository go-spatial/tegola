package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// rows affected.
const (
	raTBD             = -1
	raSuccessNoInfo   = -2
	raExecutionFailed = -3
)

// rowsAffected represents a rows affected part.
type rowsAffected struct {
	rows []int32
}

func (r rowsAffected) String() string {
	return fmt.Sprintf("%v", r.rows)
}

func (r *rowsAffected) decodeNumArg(dec *encoding.Decoder, numArg int) error {
	r.rows = resizeSlice(r.rows, numArg)

	for i := range numArg {
		r.rows[i] = dec.Int32()
	}
	return dec.Error()
}

// Total return the total number of all affected rows.
func (r rowsAffected) Total() int64 {
	total := int64(0)
	for _, rows := range r.rows {
		if rows > 0 { // add only positive number / negatives are status / error values (see above)
			total += int64(rows)
		}
	}
	return total
}
