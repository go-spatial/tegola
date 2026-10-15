//go:build go1.27

package driver

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
	"github.com/SAP/go-hdb/driver/internal/unsafe"
)

var (
	lobReflectType        = reflect.TypeFor[Lob]()
	nullLobReflectType    = reflect.TypeFor[NullLob]()
	lobPtrReflectType     = reflect.TypeFor[*Lob]()
	nullLobPtrReflectType = reflect.TypeFor[*NullLob]()
)

// scanColumn scans a single column value into the destination. Lob values are scanned
// directly into the destination, all other values are converted via convertAssign
// (no-copy for string and []byte values).
func scanColumn(scanCtx driver.ScanContext, session *session, isLob bool, dest any, src driver.Value) error {
	if isLob {
		return convertAssignLob(scanCtx, session, dest, src)
	}
	return convertAssign(scanCtx, dest, src)
}

func convertAssign(scanCtx driver.ScanContext, dest, src any) error {
	switch src := src.(type) {
	case []byte:
		switch d := dest.(type) {
		case *[]byte:
			*d = src
			return nil
		case *sql.RawBytes:
			*d = src
			return nil
		case *string:
			*d = unsafe.ByteSlice2String(src)
			return nil
		}
	case string:
		switch d := dest.(type) {
		case *string:
			*d = src
			return nil
		case *sql.RawBytes:
			*d = unsafe.String2ByteSlice(src)
			return nil
		case *[]byte:
			*d = unsafe.String2ByteSlice(src)
			return nil
		}
	default:
		return sql.ConvertAssign(scanCtx, dest, src)
	}

	// reflect tail: string- and []byte-kind named types and pointer chains,
	// aliasing the driver-owned source buffer (never copied).
	dpv := reflect.ValueOf(dest)
	if dpv.Kind() != reflect.Pointer {
		return sql.ConvertAssign(scanCtx, dest, src)
	}
	dv := reflect.Indirect(dpv)
	for dv.Kind() == reflect.Pointer {
		if dv.IsNil() {
			dv.Set(reflect.New(dv.Type().Elem()))
		}
		dv = dv.Elem()
	}
	switch dv.Kind() {
	case reflect.String:
		if s, ok := src.(string); ok {
			dv.SetString(s)
		} else {
			dv.SetString(unsafe.ByteSlice2String(src.([]byte)))
		}
		return nil
	case reflect.Slice:
		if dv.Type().Elem().Kind() == reflect.Uint8 {
			if b, ok := src.([]byte); ok {
				dv.SetBytes(b)
			} else {
				dv.SetBytes(unsafe.String2ByteSlice(src.(string)))
			}
			return nil
		}
	}
	return sql.ConvertAssign(scanCtx, dest, src)
}

func convertAssignLob(scanCtx driver.ScanContext, session *session, dest, src any) error {
	if src == nil {
		// for compatibility with the previous behavior, a NULL value is handled
		// by the default conversion: it does not allocate nested pointers.
		if d, ok := dest.(*NullLob); ok { // NullLob does not implement the Scanner interface in this build
			d.Valid = false
			return nil
		}
		return sql.ConvertAssign(scanCtx, dest, src)
	}
	switch d := dest.(type) {
	case *string:
		switch s := src.(type) {
		case *p.LobOutDescr:
			b := unsafe.String2ByteSlice(*d)[:0]
			wr := (*byteSliceWriter)(&b)
			err := session.readLobComplete(s, wr)
			*d = unsafe.ByteSlice2String(b)
			return err
		case string: // mock source
			*d = s
			return nil
		case []byte: // mock source
			*d = string(s)
			return nil
		}
	case *[]byte:
		switch s := src.(type) {
		case *p.LobOutDescr:
			*d = (*d)[:0]
			wr := (*byteSliceWriter)(d)
			return session.readLobComplete(s, wr)
		case string: // mock source
			*d = []byte(s)
			return nil
		case []byte: // mock source
			*d = s
			return nil
		}
	case io.Writer:
		switch s := src.(type) {
		case *p.LobOutDescr:
			return session.readLobComplete(s, d)
		case string: // mock source
			_, err := d.Write([]byte(s))
			return err
		case []byte: // mock source
			_, err := d.Write(s)
			return err
		}
	case *Lob:
		if d.wr == nil {
			d.wr = new(bytes.Buffer)
		}
		switch s := src.(type) {
		case *p.LobOutDescr:
			return session.readLobComplete(s, d.wr)
		case string: // mock source
			_, err := d.wr.Write([]byte(s))
			return err
		case []byte: // mock source
			_, err := d.wr.Write(s)
			return err
		}
	case *NullLob:
		if d.Lob == nil {
			d.Lob = new(Lob)
		}
		if d.Lob.wr == nil {
			d.Lob.wr = new(bytes.Buffer)
		}
		d.Valid = true
		switch s := src.(type) {
		case *p.LobOutDescr:
			return session.readLobComplete(s, d.Lob.wr)
		case string: // mock source
			_, err := d.Lob.wr.Write([]byte(s))
			return err
		case []byte: // mock source
			_, err := d.Lob.wr.Write(s)
			return err
		}
	case *sql.Null[Lob]:
		if d.V.wr == nil {
			d.V.wr = new(bytes.Buffer)
		}
		d.Valid = true
		switch s := src.(type) {
		case *p.LobOutDescr:
			return session.readLobComplete(s, d.V.wr)
		case string: // mock source
			_, err := d.V.wr.Write([]byte(s))
			return err
		case []byte: // mock source
			_, err := d.V.wr.Write(s)
			return err
		}
	case *sql.Null[*Lob]:
		if d.V == nil {
			d.V = new(Lob)
		}
		if d.V.wr == nil {
			d.V.wr = new(bytes.Buffer)
		}
		d.Valid = true
		switch s := src.(type) {
		case *p.LobOutDescr:
			return session.readLobComplete(s, d.V.wr)
		case string: // mock source
			_, err := d.V.wr.Write([]byte(s))
			return err
		case []byte: // mock source
			_, err := d.V.wr.Write(s)
			return err
		}
	}

	dpv := reflect.ValueOf(dest)
	if dpv.Kind() != reflect.Pointer {
		return errors.New("destination not a pointer")
	}

	dv := reflect.Indirect(dpv)

	// dereference nested pointers, allocating intermediate values if necessary
	if dv.Kind() == reflect.Pointer {
		if dv.IsNil() {
			dv.Set(reflect.New(dv.Type().Elem()))
		}
		return convertAssignLob(scanCtx, session, dv.Interface(), src)
	}

	// support database/sql sql.Null[T] destinations
	if isGenNull, _ := isGenericNull(dv.Type()); isGenNull {
		vField := dv.FieldByName("V")
		validField := dv.FieldByName("Valid")
		if err := convertAssignLob(scanCtx, session, vField.Addr().Interface(), src); err != nil {
			return err
		}
		validField.SetBool(true)
		return nil
	}

	// support structs embedding Lob or NullLob (by value or by pointer)
	// stream directly into the embedded value, avoiding the buffering fallback below
	// (vflow embedding pattern go < 1.27)
	if dv.Kind() == reflect.Struct && dv.CanAddr() {
		for f, fv := range dv.Fields() {
			if !f.Anonymous {
				continue
			}
			switch f.Type {
			case lobReflectType, nullLobReflectType:
				return convertAssignLob(scanCtx, session, fv.Addr().Interface(), src)
			case lobPtrReflectType, nullLobPtrReflectType:
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				return convertAssignLob(scanCtx, session, fv.Interface(), src)
			}
		}
	}

	if descr, ok := src.(*p.LobOutDescr); ok {
		wr := &bytes.Buffer{}
		if err := session.readLobComplete(descr, wr); err != nil {
			return err
		}
		return sql.ConvertAssign(scanCtx, dest, wr.Bytes())
	}

	return sql.ConvertAssign(scanCtx, dest, src)
}
