// Package auth provides authentication methods.
package auth

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

/*
authentication method types supported by the driver:
  - basic authentication (username, password based) (whether SCRAMSHA256 or SCRAMPBKDF2SHA256) and
  - X509 (client certificate) authentication and
  - JWT (token) authentication
*/
const (
	MtSCRAMSHA256       = "SCRAMSHA256"       // password
	MtSCRAMPBKDF2SHA256 = "SCRAMPBKDF2SHA256" // password pbkdf2
	MtX509              = "X509"              // client certificate
	MtJWT               = "JWT"               // json web token
	MtSessionCookie     = "SessionCookie"     // session cookie
	MtLDAP              = "LDAP"              // LDAP authentication
)

// authentication method orders.
const (
	MoSessionCookie byte = iota
	MoX509
	MoJWT
	MoSCRAMPBKDF2SHA256
	MoSCRAMSHA256
	MoLDAP
)

// A Method defines the interface for an authentication method.
type Method interface {
	fmt.Stringer
	Typ() string
	Order() byte
	PrepareInitReq(prms *Prms) error
	InitRepDecode(d *encoding.Decoder) error
	PrepareFinalReq(prms *Prms) error
	FinalRepDecode(d *encoding.Decoder) error
}

// Methods defines a collection of methods.
type Methods map[string]Method // key equals authentication method type.

// Order returns an ordered method slice.
func (m Methods) Order() []Method {
	methods := make([]Method, 0, len(m))
	for _, e := range m {
		methods = append(methods, e)
	}
	slices.SortFunc(methods, func(m1, m2 Method) int { return cmp.Compare(m1.Order(), m2.Order()) })
	return methods
}

// CookieGetter is implemented by authentication methods supporting cookies to reconnect.
type CookieGetter interface {
	Cookie() (logonname string, cookie []byte)
}

var (
	_ Method = (*SCRAMSHA256)(nil)
	_ Method = (*SCRAMPBKDF2SHA256)(nil)
	_ Method = (*JWT)(nil)
	_ Method = (*X509)(nil)
	_ Method = (*SessionCookie)(nil)
	_ Method = (*LDAP)(nil)
)

// Prms represents authentication parameters.
type Prms struct {
	prms []any
}

func (p *Prms) String() string { return fmt.Sprintf("%v", p.prms) }

// AddCESU8String adds a CESU8 string parameter.
func (p *Prms) AddCESU8String(s string) { p.prms = append(p.prms, s) } // unicode string
func (p *Prms) addEmpty()               { p.prms = append(p.prms, []byte{}) }
func (p *Prms) addBytes(b []byte)       { p.prms = append(p.prms, b) }
func (p *Prms) addString(s string)      { p.prms = append(p.prms, []byte(s)) } // treat like bytes to distinguisch from unicode string
func (p *Prms) addPrms() *Prms {
	prms := &Prms{}
	p.prms = append(p.prms, prms)
	return prms
}

// Size returns the size in bytes of the parameters.
func (p *Prms) Size() int {
	size := encoding.SmallintFieldSize // no of parameters (2 bytes)
	for _, prm := range p.prms {
		switch prm := prm.(type) {
		case []byte:
			size += encoding.VarFieldSize(prm)
		case string:
			size += encoding.Cesu8FieldSize(prm)
		case *Prms:
			subSize := prm.Size()
			size += (subSize + encoding.AuthVarFieldSize(subSize))
		default:
			panic("invalid parameter") // should not happen
		}
	}
	return size
}

// Encode encodes the parameters.
func (p *Prms) Encode(enc *encoding.Encoder) error {
	numPrms := len(p.prms)
	if numPrms > math.MaxInt16 {
		return fmt.Errorf("invalid number of parameters %d - maximum %d", numPrms, math.MaxInt16)
	}
	enc.Int16(int16(numPrms))

	for _, e := range p.prms {
		switch e := e.(type) {
		case []byte:
			if err := enc.LIBytes(e); err != nil {
				return err
			}
		case string:
			if err := enc.CESU8LIString(e); err != nil {
				return err
			}
		case *Prms:
			subSize := e.Size()
			if err := enc.AuthVarFieldInd(subSize); err != nil {
				return err
			}
			if err := e.Encode(enc); err != nil {
				return err
			}
		default:
			panic("invalid parameter") // should not happen
		}
	}
	return nil
}

// Decode decodes the parameters.
func (p *Prms) Decode(dec *encoding.Decoder) error {
	numPrms := int(dec.Int16())
	for range numPrms {

	}
	return nil
}

// DecodeAndCheckNumPrm decodes and ckecks the number of parameters and returns an error if not equal expected, nil otherwise.
func DecodeAndCheckNumPrm(dec *encoding.Decoder, expected int) error {
	numPrm := int(dec.Int16())
	if numPrm != expected {
		return fmt.Errorf("invalid number of parameters %d - expected %d", numPrm, expected)
	}
	return nil
}

func checkAuthMethodType(mt, expected string) error {
	if mt != expected {
		return fmt.Errorf("invalid method %s - expected %s", mt, expected)
	}
	return nil
}
