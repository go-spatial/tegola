package auth

// Salted Challenge Response Authentication Mechanism (SCRAM)

import (
	"bytes"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

func scramsha256Key(password, salt []byte) ([]byte, error) {
	return scramSHA256(scramHMAC(password, salt)), nil
}

// use cache as key calculation is expensive.
var scramKeyCache = newList(3, func(k *SCRAMSHA256) ([]byte, error) {
	return scramsha256Key([]byte(k.password), k.salt)
})

// SCRAMSHA256 implements SCRAMSHA256 authentication.
type SCRAMSHA256 struct {
	username, password    string
	clientChallenge       []byte
	clientProof           []byte
	salt, serverChallenge []byte
}

// NewSCRAMSHA256 creates a new SCRAMSHA256 instance.
func NewSCRAMSHA256(username, password string) *SCRAMSHA256 {
	return &SCRAMSHA256{username: username, password: password}
}

func (a *SCRAMSHA256) String() string {
	return fmt.Sprintf("method type %s username %s clientChallenge %s clientProof %s salt %s serverChallenge %s",
		a.Typ(), trace.Cut(a.username), trace.Redacted(a.clientChallenge), trace.Redacted(a.clientProof), trace.Redacted(a.salt), trace.Redacted(a.serverChallenge))
}

// Compare implements cache.Compare interface.
func (a *SCRAMSHA256) Compare(a1 *SCRAMSHA256) bool {
	return a.password == a1.password && bytes.Equal(a.salt, a1.salt)
}

// Typ implements the Method interface.
func (a *SCRAMSHA256) Typ() string { return MtSCRAMSHA256 }

// Order implements the Method interface.
func (a *SCRAMSHA256) Order() byte { return MoSCRAMSHA256 }

// AuthLoginName implements the Method interface.
func (a *SCRAMSHA256) AuthLoginName() string { return a.username }

// EncodeInitReq implements the Method interface.
func (a *SCRAMSHA256) EncodeInitReq(prms *Prms) error {
	a.clientChallenge = scramClientChallenge()
	prms.addBytes(a.clientChallenge)
	return nil
}

// DecodeInitReq implements the Method interface.
func (a *SCRAMSHA256) DecodeInitReq(dec *encoding.Decoder) error {
	_, clientChallenge := dec.LIBytes()
	a.clientChallenge = clientChallenge
	return nil
}

// DecodeInitReply implements the Method interface.
func (a *SCRAMSHA256) DecodeInitReply(dec *encoding.Decoder) error {
	dec.AuthVarFieldInd() // sub parameters
	if err := DecodeAndCheckNumPrm(dec, 2); err != nil {
		return err
	}
	a.salt = dec.AuthBytes()
	a.serverChallenge = dec.AuthBytes()
	if err := scramCheckSalt(a.salt); err != nil {
		return err
	}
	if err := scramCheckServerChallenge(a.serverChallenge); err != nil {
		return err
	}
	return nil
}

// EncodeFinalReq implements the Method interface.
func (a *SCRAMSHA256) EncodeFinalReq(prms *Prms) error {
	key, err := scramKeyCache.Get(a)
	if err != nil {
		return err
	}
	clientProof, err := scramClientProof(key, a.salt, a.serverChallenge, a.clientChallenge)
	if err != nil {
		return err
	}

	subPrms := prms.addPrms()
	subPrms.addBytes(clientProof)

	return nil
}

// DecodeFinalReq implements the Method interface.
func (a *SCRAMSHA256) DecodeFinalReq(dec *encoding.Decoder, logonname string) error {
	a.username = logonname
	_, b := dec.LIBytes() // sub parameters
	sub := encoding.NewDecoder(b, nil)
	if err := DecodeAndCheckNumPrm(sub, 1); err != nil {
		return err
	}
	_, a.clientProof = sub.LIBytes()
	return nil
}

// DecodeFinalReply implements the Method interface.
func (a *SCRAMSHA256) DecodeFinalReply(dec *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(dec, 2); err != nil {
		return err
	}
	mt := dec.AuthString()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	if dec.AuthVarFieldInd() == 0 { // mnSCRAMSHA256: server does not return server proof parameter
		return nil
	}
	if err := DecodeAndCheckNumPrm(dec, 1); err != nil {
		return err
	}
	serverProof := dec.AuthBytes()
	if err := scramVerifyServerProof(scramHMAC([]byte(a.password), a.salt), a.salt, a.serverChallenge, a.clientChallenge, serverProof); err != nil {
		return err
	}
	return nil
}
