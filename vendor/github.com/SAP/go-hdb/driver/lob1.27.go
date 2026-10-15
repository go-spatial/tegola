//go:build go1.27

package driver

import (
	"bytes"
	"fmt"
)

// Scan implements the database/sql/Scanner interface.
func (l *Lob) Scan(src any) error {
	/*
		starting with go1.27 fallback method only
	*/
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("lob: invalid scan type %T", src)
	}
	if l.wr == nil {
		l.wr = bytes.NewBuffer(b)
		return nil
	}
	_, err := l.wr.Write(b)
	return err
}
