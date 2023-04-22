package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/auth"
	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/protocol/x509"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// AuthPasswordSetter is implemented by authentication methods supporting password updates.
type AuthPasswordSetter interface {
	SetPassword(string)
}

// AuthTokenSetter is implemented by authentication methods supporting token updates.
type AuthTokenSetter interface {
	SetToken(string)
}

// AuthCertKeySetter is implemented by authentication methods supporting certificate and key updates.
type AuthCertKeySetter interface {
	SetCertKey(certKey *x509.CertKey)
}

// AuthCookieGetter is implemented by authentication methods supporting cookies to reconnect.
type AuthCookieGetter interface {
	Cookie() (logonname string, cookie []byte)
}

type authMethods map[string]auth.Method // key equals authentication method type.

func (m authMethods) order() []auth.Method {
	methods := maps.Values(m)
	slices.SortFunc(methods, func(a, b auth.Method) bool { return a.Order() < b.Order() })
	return methods
}

// Auth holds the client authentication methods dependant on the driver.Connector attributes.
type Auth struct {
	logonname string
	methods   authMethods
	method    auth.Method // selected method
}

// NewAuth creates a new Auth instance.
func NewAuth(logonname string) *Auth { return &Auth{logonname: logonname, methods: authMethods{}} }

func (a *Auth) String() string { return fmt.Sprintf("logonname %s", a.logonname) }

// AddSessionCookie adds session cookie authentication method.
func (a *Auth) AddSessionCookie(cookie []byte, logonname, clientID string) {
	a.methods[auth.MtSessionCookie] = auth.NewSessionCookie(cookie, logonname, clientID)
}

// AddBasic adds basic authentication methods.
func (a *Auth) AddBasic(username, password string) {
	a.methods[auth.MtSCRAMPBKDF2SHA256] = auth.NewSCRAMPBKDF2SHA256(username, password)
	a.methods[auth.MtSCRAMSHA256] = auth.NewSCRAMSHA256(username, password)
}

// AddJWT adds JWT authentication method.
func (a *Auth) AddJWT(token string) { a.methods[auth.MtJWT] = auth.NewJWT(token) }

// AddX509 adds X509 authentication method.
func (a *Auth) AddX509(certKey *x509.CertKey) { a.methods[auth.MtX509] = auth.NewX509(certKey) }

// Method returns the selected authentication method.
func (a *Auth) Method() auth.Method { return a.method }

func (a *Auth) setMethod(mt string) error {
	var ok bool
	if a.method, ok = a.methods[mt]; !ok {
		return fmt.Errorf("invalid method type: %s", mt)
	}
	return nil
}

// InitRequest returns the init request part.
func (a *Auth) InitRequest() (*AuthInitRequest, error) {
	prms := &auth.Prms{}
	prms.AddCESU8String(a.logonname)
	for _, m := range a.methods.order() {
		if err := m.PrepareInitReq(prms); err != nil {
			return nil, err
		}
	}
	return &AuthInitRequest{prms: prms}, nil
}

// InitReply returns the init reply part.
func (a *Auth) InitReply() (*AuthInitReply, error) { return &AuthInitReply{auth: a}, nil }

// FinalRequest returns the final request part.
func (a *Auth) FinalRequest() (*AuthFinalRequest, error) {
	prms := &auth.Prms{}
	if err := a.method.PrepareFinalReq(prms); err != nil {
		return nil, err
	}
	return &AuthFinalRequest{prms}, nil
}

// FinalReply returns the final reply part.
func (a *Auth) FinalReply() (*AuthFinalReply, error) { return &AuthFinalReply{method: a.method}, nil }

// AuthInitRequest represents an authentication initial request.
type AuthInitRequest struct {
	prms *auth.Prms
}

func (r *AuthInitRequest) String() string { return r.prms.String() }
func (r *AuthInitRequest) size() int      { return r.prms.Size() }
func (r *AuthInitRequest) decode(dec *encoding.Decoder, ph *PartHeader) error {
	panic("not implemented yet")
}
func (r *AuthInitRequest) encode(enc *encoding.Encoder) error { return r.prms.Encode(enc) }

// AuthInitReply represents an authentication initial reply.
type AuthInitReply struct {
	auth *Auth
}

func (r *AuthInitReply) String() string { return r.auth.String() }
func (r *AuthInitReply) decode(dec *encoding.Decoder, ph *PartHeader) error {
	d := auth.NewDecoder(dec)

	if err := d.NumPrm(2); err != nil {
		return err
	}
	mt := d.String()

	if err := r.auth.setMethod(mt); err != nil {
		return err
	}
	if err := r.auth.method.InitRepDecode(d); err != nil {
		return err
	}
	return dec.Error()
}

// AuthFinalRequest represents an authentication final request.
type AuthFinalRequest struct {
	prms *auth.Prms
}

func (r *AuthFinalRequest) String() string { return r.prms.String() }
func (r *AuthFinalRequest) size() int      { return r.prms.Size() }
func (r *AuthFinalRequest) decode(dec *encoding.Decoder, ph *PartHeader) error {
	panic("not implemented yet")
}
func (r *AuthFinalRequest) encode(enc *encoding.Encoder) error { return r.prms.Encode(enc) }

// AuthFinalReply represents an authentication final reply.
type AuthFinalReply struct {
	method auth.Method
}

func (r *AuthFinalReply) String() string { return r.method.String() }
func (r *AuthFinalReply) decode(dec *encoding.Decoder, ph *PartHeader) error {
	if err := r.method.FinalRepDecode(auth.NewDecoder(dec)); err != nil {
		return err
	}
	return dec.Error()
}
