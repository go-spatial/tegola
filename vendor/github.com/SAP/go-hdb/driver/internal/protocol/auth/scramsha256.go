package auth

// Salted Challenge Response Authentication Mechanism (SCRAM)

import (
	"bytes"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
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
	salt, serverChallenge []byte
	serverProof           []byte
}

// NewSCRAMSHA256 creates a new authSCRAMSHA256 instance.
func NewSCRAMSHA256(username, password string) *SCRAMSHA256 {
	return &SCRAMSHA256{username: username, password: password, clientChallenge: scramClientChallenge()}
}

func (a *SCRAMSHA256) String() string {
	return fmt.Sprintf("method type %s clientChallenge %v", a.Typ(), a.clientChallenge)
}

// Compare implements cache.Compare interface.
func (a *SCRAMSHA256) Compare(a1 *SCRAMSHA256) bool {
	return a.password == a1.password && bytes.Equal(a.salt, a1.salt)
}

// Typ implements the Method interface.
func (a *SCRAMSHA256) Typ() string { return MtSCRAMSHA256 }

// Order implements the Method interface.
func (a *SCRAMSHA256) Order() byte { return MoSCRAMSHA256 }

// PrepareInitReq implements the Method interface.
func (a *SCRAMSHA256) PrepareInitReq(prms *Prms) error {
	prms.addString(a.Typ())
	prms.addBytes(a.clientChallenge)
	return nil
}

// InitRepDecode implements the Method interface.
func (a *SCRAMSHA256) InitRepDecode(d *encoding.Decoder) error {
	d.AuthVarFieldInd() // sub parameters
	if err := DecodeAndCheckNumPrm(d, 2); err != nil {
		return err
	}
	a.salt = d.AuthBytes()
	a.serverChallenge = d.AuthBytes()
	if err := scramCheckSalt(a.salt); err != nil {
		return err
	}
	if err := scramCheckServerChallenge(a.serverChallenge); err != nil {
		return err
	}
	return nil
}

// PrepareFinalReq implements the Method interface.
func (a *SCRAMSHA256) PrepareFinalReq(prms *Prms) error {
	key, err := scramKeyCache.Get(a)
	if err != nil {
		return err
	}
	clientProof, err := scramClientProof(key, a.salt, a.serverChallenge, a.clientChallenge)
	if err != nil {
		return err
	}

	prms.AddCESU8String(a.username)
	prms.addString(a.Typ())
	subPrms := prms.addPrms()
	subPrms.addBytes(clientProof)

	return nil
}

// FinalRepDecode implements the Method interface.
func (a *SCRAMSHA256) FinalRepDecode(d *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(d, 2); err != nil {
		return err
	}
	mt := d.AuthString()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	if d.AuthVarFieldInd() == 0 { // mnSCRAMSHA256: server does not return server proof parameter
		return nil
	}
	if err := DecodeAndCheckNumPrm(d, 1); err != nil {
		return err
	}
	a.serverProof = d.AuthBytes()
	return nil
}
