package protocol

import (
	"fmt"
	"strings"

	"github.com/SAP/go-hdb/driver/internal/protocol/auth"
	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

// AuthHnd holds the client authentication methods dependent on the driver.Connector attributes and handles the authentication hdb protocol.
type AuthHnd struct {
	logonname string
	methods   auth.Methods
	selected  auth.Method // selected method
}

// NewAuthHnd creates a new AuthHnd instance.
func NewAuthHnd(logonname string) *AuthHnd {
	return &AuthHnd{logonname: logonname, methods: auth.Methods{}}
}

func (a *AuthHnd) String() string { return "logonname " + a.logonname }

// AddSessionCookie adds session cookie authentication method.
func (a *AuthHnd) AddSessionCookie(cookie []byte, logonname, clientID string) {
	a.methods[auth.MtSessionCookie] = auth.NewSessionCookie(cookie, logonname, clientID)
}

// AddBasic adds basic authentication methods.
func (a *AuthHnd) AddBasic(username, password string) {
	a.methods[auth.MtSCRAMPBKDF2SHA256] = auth.NewSCRAMPBKDF2SHA256(username, password)
	a.methods[auth.MtSCRAMSHA256] = auth.NewSCRAMSHA256(username, password)
}

// AddJWT adds JWT authentication method.
func (a *AuthHnd) AddJWT(token string) { a.methods[auth.MtJWT] = auth.NewJWT(token) }

// AddX509 adds X509 authentication method.
func (a *AuthHnd) AddX509(certKey *auth.CertKey) { a.methods[auth.MtX509] = auth.NewX509(certKey) }

/*
AddLDAP adds LDAP authentication method.

The LDAP method does not authenticate the server: the server-provided RSA
public key is trusted as is. LDAP authentication should therefore only be used
over a TLS transport (see SessionTLS/SetTLS).
*/
func (a *AuthHnd) AddLDAP(username, password string) {
	a.methods[auth.MtLDAP] = auth.NewLDAP(username, password)
}

// Selected returns the selected authentication method.
func (a *AuthHnd) Selected() auth.Method { return a.selected }

func (a *AuthHnd) setMethod(mt string) error {
	var ok bool
	if a.selected, ok = a.methods[mt]; !ok {
		return fmt.Errorf("invalid method type: %s", mt)
	}
	return nil
}

// InitRequest returns the init request part holding the logonname and the
// offered authentication methods.
func (a *AuthHnd) InitRequest() (*AuthInitRequest, error) {
	return &AuthInitRequest{logonname: a.logonname, methods: a.methods.Order()}, nil
}

// InitReply returns the init reply part.
func (a *AuthHnd) InitReply() (*AuthInitReply, error) { return &AuthInitReply{authHnd: a}, nil }

// FinalRequest returns the final request part holding the selected method.
func (a *AuthHnd) FinalRequest() (*AuthFinalRequest, error) {
	return &AuthFinalRequest{method: a.selected}, nil
}

// FinalReply returns the final reply part.
func (a *AuthHnd) FinalReply() (*AuthFinalReply, error) {
	return &AuthFinalReply{Method: a.selected}, nil
}

// AuthInitRequest represents an authentication initial request. The generic
// part holds the logonname and the offered authentication methods, each of
// which holds its own detail parameters. Encode marshals the struct into the
// wire parameter list, decode unmarshals the wire parameter list into the
// struct.
type AuthInitRequest struct {
	logonname string
	methods   []auth.Method // wire-interpreted offer methods, set on decode
}

// encode encodes the request by iterating the offered methods: the method
// name, then the method's own detail parameters.
func (r *AuthInitRequest) encode(enc *encoding.Encoder) error {
	prms := new(auth.Prms)
	prms.AddCESU8String(r.logonname)
	for _, m := range r.methods {
		prms.AddString(m.Typ()) // generic part: method name
		if err := m.EncodeInitReq(prms); err != nil {
			return err
		}
	}
	return prms.Encode(enc)
}

func (r *AuthInitRequest) decode(dec *encoding.Decoder, _ *PartHeader, attrs *ReaderAttrs) error {
	numPrm := int(dec.Int16())
	if numPrm < 1 {
		return fmt.Errorf("invalid number of parameters %d - expected at least 1", numPrm)
	}
	if (numPrm-1)%2 != 0 {
		return fmt.Errorf("invalid number of parameters %d - expected pairs of method name and detail", numPrm)
	}
	_, logonname, err := dec.CESU8LIString()
	if err != nil {
		return err
	}
	r.logonname = logonname

	methods := make([]auth.Method, 0, numPrm/2)
	for i := 1; i+1 < numPrm; i += 2 {
		_, mt := dec.LIString()
		m, ok := auth.NewMethod(mt)
		if !ok {
			return fmt.Errorf("unknown authentication method %s", mt)
		}
		if err := m.DecodeInitReq(dec); err != nil {
			return err
		}
		methods = append(methods, m)
	}
	r.methods = methods
	return nil
}

func (r *AuthInitRequest) String() string {
	var b strings.Builder
	if r.logonname != "" {
		fmt.Fprintf(&b, "logonname %s", trace.Cut(r.logonname))
	}
	if r.methods == nil {
		if b.Len() != 0 {
			b.WriteString("; ")
		}
		b.WriteString("method segments not decodable")
		return b.String()
	}
	for _, m := range r.methods {
		if b.Len() != 0 {
			b.WriteString("; ")
		}
		b.WriteString(m.String())
	}
	return b.String()
}

// Logonname returns the logonname, empty string if not present.
func (r *AuthInitRequest) Logonname() string { return r.logonname }

// AuthInitReply represents an authentication initial reply.
type AuthInitReply struct {
	authHnd *AuthHnd    // client side, may be nil on the sniffer side
	Method  auth.Method // wire-interpreted selected method, set on decode without authHnd
}

func (r *AuthInitReply) String() string {
	if r.Method != nil {
		return r.Method.String()
	}
	if r.authHnd == nil {
		return "method not decodable"
	}
	if r.authHnd.Selected() != nil {
		return r.authHnd.Selected().String()
	}
	return r.authHnd.String()
}
func (r *AuthInitReply) decode(dec *encoding.Decoder, _ *PartHeader, _ *ReaderAttrs) error {
	if err := auth.DecodeAndCheckNumPrm(dec, 2); err != nil {
		return err
	}
	if r.authHnd != nil {
		mt := dec.AuthString()
		if err := r.authHnd.setMethod(mt); err != nil {
			return err
		}
		return r.authHnd.selected.DecodeInitReply(dec)
	}
	m, err := auth.InitRepMethod(dec)
	if err != nil {
		return err
	}
	if err := m.DecodeInitReply(dec); err != nil {
		return err
	}
	r.Method = m
	return nil
}

// AuthFinalRequest represents an authentication final request. The generic
// part holds the selected method with its detail parameters. Encode
// marshals the struct into the wire parameter list, decode unmarshals the
// wire parameter list into the struct.
type AuthFinalRequest struct {
	method auth.Method // the selected method: the client's prepared, the sniffer's wire-interpreted
}

func (r *AuthFinalRequest) encode(enc *encoding.Encoder) error {
	prms := new(auth.Prms)
	prms.AddCESU8String(r.method.AuthLoginName())
	prms.AddString(r.method.Typ())
	if err := r.method.EncodeFinalReq(prms); err != nil {
		return err
	}
	return prms.Encode(enc)
}

func (r *AuthFinalRequest) String() string {
	if r.method == nil {
		return "method not decodable"
	}
	return r.method.String()
}

func (r *AuthFinalRequest) decode(dec *encoding.Decoder, _ *PartHeader, attrs *ReaderAttrs) error {
	numPrm := int(dec.Int16())
	if numPrm < 3 {
		return fmt.Errorf("invalid number of parameters %d - expected at least 3", numPrm)
	}
	_, logonname, err := dec.CESU8LIString()
	if err != nil {
		return err
	}
	_, mt := dec.LIString()
	m, ok := auth.NewMethod(mt)
	if !ok {
		return fmt.Errorf("unknown authentication method %s", mt)
	}
	if err := m.DecodeFinalReq(dec, logonname); err != nil {
		return err
	}
	r.method = m
	return nil
}

// AuthFinalReply represents an authentication final reply.
type AuthFinalReply struct {
	Method auth.Method // the selected method: the client's prepared, the sniffer's wire-interpreted
}

func (r *AuthFinalReply) String() string {
	if r.Method == nil {
		return "method not decodable"
	}
	return r.Method.String()
}
func (r *AuthFinalReply) decode(dec *encoding.Decoder, _ *PartHeader, attrs *ReaderAttrs) error {
	if r.Method == nil {
		return nil
	}

	return r.Method.DecodeFinalReply(dec)
}
