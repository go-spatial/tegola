package driver

import (
	"database/sql/driver"
	"io"
)

// byteSliceWriter implements io.Writer by appending to a byte slice.
type byteSliceWriter []byte

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	*w = append(*w, p...)
	return len(p), nil
}

// A Lob is the driver representation of a database large object field.
// A Lob object uses an io.Reader object as source for writing content to a database lob field.
// A Lob object uses an io.Writer object as destination for reading content from a database lob field.
// A Lob can be created with the NewLob constructor with io.Reader and io.Writer as parameters or
// with new, setting io.Reader and io.Writer with the SetReader and SetWriter methods.
type Lob struct {
	rd io.Reader
	wr io.Writer
}

// NewLob creates a new Lob instance with the io.Reader and io.Writer given as parameters.
func NewLob(rd io.Reader, wr io.Writer) *Lob {
	return &Lob{rd: rd, wr: wr}
}

// Reader returns the io.Reader of the Lob.
func (l Lob) Reader() io.Reader {
	return l.rd
}

// SetReader sets the io.Reader source for a lob field to be written to the database
// and returns *Lob, to enable simple call chaining.
func (l *Lob) SetReader(rd io.Reader) *Lob {
	l.rd = rd
	return l
}

// Writer returns the io.Writer of the Lob.
func (l Lob) Writer() io.Writer {
	return l.wr
}

// SetWriter sets the io.Writer destination for a lob field to be read from the database
// and returns *Lob, to enable simple call chaining.
func (l *Lob) SetWriter(wr io.Writer) *Lob {
	l.wr = wr
	return l
}

// NullLob represents a Lob that may be null.
// NullLob implements the Scanner interface so
// it can be used as a scan destination, similar to NullString.
type NullLob struct {
	Lob   *Lob
	Valid bool // Valid is true if Lob is not NULL
}

// Scan implements the database/sql/Scanner interface.
func (n *NullLob) Scan(value any) error {
	/*
		starting with go1.27 fallback method only
	*/
	/*
		In contrast to the Null[T] Scan implementation we do not
		create a new lob instance in case of value == nil to
		enable reuse of n.Lob.

		func (n *Null[T]) Scan(value any) error {
			if value == nil {
				n.V, n.Valid = *new(T), false
				return nil
			}
			n.Valid = true
			return convertAssign(&n.V, value)
		}
	*/
	if value == nil {
		n.Valid = false
		return nil
	}
	if n.Lob == nil {
		n.Lob = new(Lob)
	}
	n.Valid = true
	return n.Lob.Scan(value)
}

// Value implements the database/sql/Valuer interface.
func (n NullLob) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Lob, nil
}
