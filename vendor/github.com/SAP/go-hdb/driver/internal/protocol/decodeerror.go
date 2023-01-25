package protocol

import "fmt"

// DecodeError represents a decoding error.
type DecodeError struct {
	row       int
	fieldName string
	s         string // error text
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode error: %s row: %d fieldname: %s", e.s, e.row, e.fieldName)
}

// DecodeErrors represents a list of decoding errors.
type DecodeErrors []*DecodeError

// RowError returns an error if one is assigned to a row, nil otherwise.
func (errors DecodeErrors) RowError(row int) error {
	for _, err := range errors {
		if err.row == row {
			return err
		}
	}
	return nil
}
