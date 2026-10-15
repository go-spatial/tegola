package auth

import (
	"errors"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

// JWT implements JWT authentication.
type JWT struct {
	token     string
	logonname string
	_cookie   []byte
}

// NewJWT creates a new JWT instance.
func NewJWT(token string) *JWT { return &JWT{token: token} }

func (a *JWT) String() string {
	return fmt.Sprintf("method type %s logonname %s token %s cookie %s",
		a.Typ(), trace.Cut(a.logonname), trace.Redacted(a.token), trace.Redacted(a._cookie))
}

// Cookie implements the AuthCookieGetter interface.
func (a *JWT) Cookie() (string, []byte) { return a.logonname, a._cookie }

// Typ implements the Method interface.
func (a *JWT) Typ() string { return MtJWT }

// Order implements the Method interface.
func (a *JWT) Order() byte { return MoJWT }

// AuthLoginName implements the Method interface.
func (a *JWT) AuthLoginName() string { return a.logonname }

// EncodeInitReq implements the Method interface.
func (a *JWT) EncodeInitReq(prms *Prms) error {
	prms.AddString(a.token)
	return nil
}

// DecodeInitReq implements the Method interface.
func (a *JWT) DecodeInitReq(dec *encoding.Decoder) error {
	_, token := dec.LIBytes()
	a.token = string(token)
	return nil
}

// DecodeInitReply implements the Method interface.
func (a *JWT) DecodeInitReply(dec *encoding.Decoder) error {
	a.logonname = dec.AuthString()
	return nil
}

// EncodeFinalReq implements the Method interface.
func (a *JWT) EncodeFinalReq(prms *Prms) error {
	prms.addEmpty() // empty parameter
	return nil
}

// DecodeFinalReq implements the Method interface.
func (a *JWT) DecodeFinalReq(dec *encoding.Decoder, logonname string) error {
	a.logonname = logonname
	_, empty := dec.LIBytes()
	if len(empty) != 0 {
		return errors.New("expected empty parameter")
	}
	return nil
}

// DecodeFinalReply implements the Method interface.
func (a *JWT) DecodeFinalReply(dec *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(dec, 2); err != nil {
		return err
	}
	mt := dec.AuthString()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	a._cookie = dec.AuthBytes()
	return nil
}
