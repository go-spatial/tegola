//go:build !go1.27

package driver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
	"github.com/SAP/go-hdb/driver/internal/unsafe"
)

func scanLob(src any, wr io.Writer) error {
	switch src := src.(type) {

	// standard case with go-hdb connected to HANA
	case p.LobScanner:
		if err := src.Scan(wr); err != nil {
			var dbErr Error
			if errors.As(err, &dbErr) && dbErr.Code() == p.HdbErrWhileParsingProtocol {
				return errInvalidLobLocatorID
			}
			return err
		}
		return nil

	default:
		return fmt.Errorf("lob: invalid scan type %T", src)

	// the following cases do support types which might be used in
	// db mock scenarios
	case string:
		_, err := io.Copy(wr, strings.NewReader(src))
		return err

	case []byte:
		_, err := io.Copy(wr, bytes.NewReader(src))
		return err

	case io.Reader:
		_, err := io.Copy(wr, src)
		return err
	}
}

// ScanLobBytes supports scanning Lob data into a byte slice.
// This enables using []byte based custom types for scanning Lobs instead of using a Lob object.
// For usage please refer to the example.
func ScanLobBytes(src any, b *[]byte) error {
	if b == nil {
		return fmt.Errorf("lob scan error: parameter b %T is nil", b)
	}
	*b = (*b)[:0]
	return scanLob(src, (*byteSliceWriter)(b))
}

// ScanLobString supports scanning Lob data into a string.
// This enables using string based custom types for scanning Lobs instead of using a Lob object.
// For usage please refer to the example.
func ScanLobString(src any, s *string) error {
	if s == nil {
		return fmt.Errorf("lob scan error: parameter s %T is nil", s)
	}
	b := unsafe.String2ByteSlice(*s)
	b = b[:0]
	if err := scanLob(src, (*byteSliceWriter)(&b)); err != nil {
		return err
	}
	*s = unsafe.ByteSlice2String(b)
	return nil
}

// ScanLobWriter supports scanning Lob data into an io.Writer object.
// This enables using io.Writer based custom types for scanning Lobs instead of using a Lob object.
// For usage please refer to the example.
func ScanLobWriter(src any, wr io.Writer) error {
	if wr == nil {
		return fmt.Errorf("lob scan error: parameter wr %T is nil", wr)
	}
	return scanLob(src, wr)
}

// Scan implements the database/sql/Scanner interface.
func (l *Lob) Scan(src any) error {
	if l.wr == nil {
		l.wr = new(bytes.Buffer)
	}
	return ScanLobWriter(src, l.wr)
}
