// Package auth provides authentication methods.
package auth

import (
	"cmp"
	"errors"
	"fmt"
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
//
// The request codecs are symmetric: the generic part of the authentication
// protocol writes and reads the common framing - parameter count, logonname
// and method name - and delegates the method specific detail parameter to the
// dedicated methods below. The detail is a single parameter, which may be a
// nested sub-parameter vector.
type Method interface {
	fmt.Stringer
	Typ() string
	Order() byte
	AuthLoginName() string
	EncodeInitReq(prms *Prms) error
	DecodeInitReq(dec *encoding.Decoder) error
	DecodeInitReply(dec *encoding.Decoder) error
	EncodeFinalReq(prms *Prms) error
	DecodeFinalReq(dec *encoding.Decoder, logonname string) error
	DecodeFinalReply(dec *encoding.Decoder) error
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

// ErrAuthVerifyFailed indicates that the server response failed client-side
// authentication verification (e.g. an invalid SCRAM server proof or an LDAP
// client challenge mismatch). It is a classification error: an authentication
// method codec fails with such an error if the corresponding verification
// cannot be performed (e.g. by a sniffer without the client credentials) and
// not because of malformed wire content.
var ErrAuthVerifyFailed = errors.New("authentication check failed")

// NewMethod returns a new authentication method of the given type used to
// interpret authentication wire content. The returned method is created
// without any client credentials.
func NewMethod(mt string) (Method, bool) {
	switch mt {
	case MtSCRAMSHA256:
		return &SCRAMSHA256{}, true
	case MtSCRAMPBKDF2SHA256:
		return &SCRAMPBKDF2SHA256{}, true
	case MtX509:
		return NewX509(&CertKey{}), true
	case MtJWT:
		return NewJWT(""), true
	case MtSessionCookie:
		return NewSessionCookie(nil, "", ""), true
	case MtLDAP:
		return NewLDAP("", ""), true
	default:
		return nil, false
	}
}

// InitRepMethod interprets the method type of an authentication initial reply
// into a credential-free method instance. The reply tail is decoded by the
// returned method's DecodeInitReply.
func InitRepMethod(dec *encoding.Decoder) (Method, error) {
	mt := dec.AuthString()
	m, ok := NewMethod(mt)
	if !ok {
		return nil, fmt.Errorf("unknown authentication method %s", mt)
	}
	return m, nil
}

// DecodeAndCheckNumPrm decodes and checks the number of parameters and returns an error if not equal expected, nil otherwise.
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
