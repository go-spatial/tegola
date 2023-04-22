package auth

// Salted Challenge Response Authentication Mechanism (SCRAM)

import (
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

// SCRAMPBKDF2SHA256 implements SCRAMPBKDF2SHA256 authentication.
type SCRAMPBKDF2SHA256 struct {
	username, password       string
	clientChallenge          []byte
	salt, serverChallenge    []byte
	clientProof, serverProof []byte
	rounds                   uint32
}

// NewSCRAMPBKDF2SHA256 creates a new authSCRAMPBKDF2SHA256 instance.
func NewSCRAMPBKDF2SHA256(username, password string) *SCRAMPBKDF2SHA256 {
	return &SCRAMPBKDF2SHA256{username: username, password: password, clientChallenge: clientChallenge()}
}

func (a *SCRAMPBKDF2SHA256) String() string {
	return fmt.Sprintf("method type %s clientChallenge %v", a.Typ(), a.clientChallenge)
}

// SetPassword implenets the AuthPasswordSetter interface.
func (a *SCRAMPBKDF2SHA256) SetPassword(password string) { a.password = password }

// Typ implements the CookieGetter interface.
func (a *SCRAMPBKDF2SHA256) Typ() string { return MtSCRAMPBKDF2SHA256 }

// Order implements the CookieGetter interface.
func (a *SCRAMPBKDF2SHA256) Order() byte { return MoSCRAMPBKDF2SHA256 }

// PrepareInitReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) PrepareInitReq(prms *Prms) error {
	prms.addString(a.Typ())
	prms.addBytes(a.clientChallenge)
	return nil
}

// InitRepDecode implements the Method interface.
func (a *SCRAMPBKDF2SHA256) InitRepDecode(d *Decoder) error {
	d.subSize() // sub parameters
	if err := d.NumPrm(3); err != nil {
		return err
	}
	a.salt = d.bytes()
	a.serverChallenge = d.bytes()
	if err := checkSalt(a.salt); err != nil {
		return err
	}
	if err := checkServerChallenge(a.serverChallenge); err != nil {
		return err
	}
	var err error
	if a.rounds, err = d.bigUint32(); err != nil {
		return err
	}
	return nil
}

// PrepareFinalReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) PrepareFinalReq(prms *Prms) error {
	key := scrampbkdf2sha256Key([]byte(a.password), a.salt, int(a.rounds))
	a.clientProof = clientProof(key, a.salt, a.serverChallenge, a.clientChallenge)
	if err := checkClientProof(a.clientProof); err != nil {
		return err
	}

	prms.AddCESU8String(a.username)
	prms.addString(a.Typ())
	subPrms := prms.addPrms()
	subPrms.addBytes(a.clientProof)

	return nil
}

// FinalRepDecode implements the Method interface.
func (a *SCRAMPBKDF2SHA256) FinalRepDecode(d *Decoder) error {
	if err := d.NumPrm(2); err != nil {
		return err
	}
	mt := d.String()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	d.subSize()
	if err := d.NumPrm(1); err != nil {
		return err
	}
	a.serverProof = d.bytes()
	return nil
}

func scrampbkdf2sha256Key(password, salt []byte, rounds int) []byte {
	return _sha256(pbkdf2.Key(password, salt, rounds, clientProofSize, sha256.New))
}
