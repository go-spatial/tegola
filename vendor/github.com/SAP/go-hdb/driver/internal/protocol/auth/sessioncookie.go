package auth

import (
	"errors"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

// SessionCookie implements session cookie authentication.
type SessionCookie struct {
	cookie    []byte
	logonname string
	clientID  string
}

// NewSessionCookie creates a new SessionCookie instance.
func NewSessionCookie(cookie []byte, logonname, clientID string) *SessionCookie {
	return &SessionCookie{cookie: cookie, logonname: logonname, clientID: clientID}
}

func (a *SessionCookie) String() string {
	return fmt.Sprintf("method type %s cookie %s logonname %s clientID %s",
		a.Typ(), trace.Redacted(a.cookie), trace.Cut(a.logonname), trace.Cut(a.clientID))
}

// Typ implements the Method interface.
func (a *SessionCookie) Typ() string { return MtSessionCookie }

// Order implements the Method interface.
func (a *SessionCookie) Order() byte { return MoSessionCookie }

// AuthLoginName implements the Method interface.
func (a *SessionCookie) AuthLoginName() string { return a.logonname }

// EncodeInitReq implements the Method interface.
func (a *SessionCookie) EncodeInitReq(prms *Prms) error {
	b := make([]byte, 0, len(a.cookie)+len(a.clientID))
	b = append(b, a.cookie...)
	b = append(b, a.clientID...)
	prms.addBytes(b) // cookie + clientID
	return nil
}

// DecodeInitReq implements the Method interface.
func (a *SessionCookie) DecodeInitReq(dec *encoding.Decoder) error {
	_, a.cookie = dec.LIBytes() // cookie + clientID
	return nil
}

// DecodeInitReply implements the Method interface.
func (a *SessionCookie) DecodeInitReply(_ *encoding.Decoder) error {
	return nil
}

// EncodeFinalReq implements the Method interface.
func (a *SessionCookie) EncodeFinalReq(prms *Prms) error {
	prms.addEmpty() // empty parameter
	return nil
}

// DecodeFinalReq implements the Method interface.
func (a *SessionCookie) DecodeFinalReq(dec *encoding.Decoder, logonname string) error {
	a.logonname = logonname
	_, empty := dec.LIBytes()
	if len(empty) != 0 {
		return errors.New("expected empty parameter")
	}
	return nil
}

// DecodeFinalReply implements the Method interface.
func (a *SessionCookie) DecodeFinalReply(dec *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(dec, 2); err != nil {
		return err
	}
	mt := dec.AuthString()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	dec.AuthBytes() // second parameter seems to be empty
	return nil
}
