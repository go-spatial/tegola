package auth

// Salted Challenge Response Authentication Mechanism (SCRAM)

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha256"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

func scrampbkdf2sha256Key(password string, salt []byte, rounds int) ([]byte, error) {
	b, err := pbkdf2.Key(sha256.New, password, salt, rounds, scramClientProofSize)
	if err != nil {
		return nil, err
	}
	return scramSHA256(b), nil
}

// use cache as key calculation is expensive.
var scrampbkdf2KeyCache = newList(3, func(k *SCRAMPBKDF2SHA256) ([]byte, error) {
	return scrampbkdf2sha256Key(k.password, k.salt, int(k.rounds))
})

// SCRAMPBKDF2SHA256 implements SCRAMPBKDF2SHA256 authentication.
type SCRAMPBKDF2SHA256 struct {
	username, password    string
	clientChallenge       []byte
	salt, serverChallenge []byte
	serverProof           []byte
	rounds                uint32
}

// NewSCRAMPBKDF2SHA256 creates a new authSCRAMPBKDF2SHA256 instance.
func NewSCRAMPBKDF2SHA256(username, password string) *SCRAMPBKDF2SHA256 {
	return &SCRAMPBKDF2SHA256{username: username, password: password, clientChallenge: scramClientChallenge()}
}

func (a *SCRAMPBKDF2SHA256) String() string {
	return fmt.Sprintf("method type %s clientChallenge %v", a.Typ(), a.clientChallenge)
}

// Compare implements cache.Compare interface.
func (a *SCRAMPBKDF2SHA256) Compare(a1 *SCRAMPBKDF2SHA256) bool {
	return a.password == a1.password && bytes.Equal(a.salt, a1.salt) && a.rounds == a1.rounds
}

// Typ implements the Method interface.
func (a *SCRAMPBKDF2SHA256) Typ() string { return MtSCRAMPBKDF2SHA256 }

// Order implements the Method interface.
func (a *SCRAMPBKDF2SHA256) Order() byte { return MoSCRAMPBKDF2SHA256 }

// PrepareInitReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) PrepareInitReq(prms *Prms) error {
	prms.addString(a.Typ())
	prms.addBytes(a.clientChallenge)
	return nil
}

// InitRepDecode implements the Method interface.
func (a *SCRAMPBKDF2SHA256) InitRepDecode(d *encoding.Decoder) error {
	d.AuthVarFieldInd() // sub parameters
	if err := DecodeAndCheckNumPrm(d, 3); err != nil {
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
	var err error
	if a.rounds, err = d.AuthBigUint32(); err != nil {
		return err
	}
	return nil
}

// PrepareFinalReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) PrepareFinalReq(prms *Prms) error {
	key, err := scrampbkdf2KeyCache.Get(a)
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
func (a *SCRAMPBKDF2SHA256) FinalRepDecode(d *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(d, 2); err != nil {
		return err
	}
	mt := d.AuthString()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	d.AuthVarFieldInd()
	if err := DecodeAndCheckNumPrm(d, 1); err != nil {
		return err
	}
	a.serverProof = d.AuthBytes()
	return nil
}
