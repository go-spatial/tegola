package auth

// Salted Challenge Response Authentication Mechanism (SCRAM)

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha256"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

func scrampbkdf2sha256SaltedPassword(password string, salt []byte, rounds int) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, rounds, scramClientProofSize)
}

func scrampbkdf2sha256Key(password string, salt []byte, rounds int) ([]byte, error) {
	b, err := scrampbkdf2sha256SaltedPassword(password, salt, rounds)
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
	clientProof           []byte
	salt, serverChallenge []byte
	rounds                uint32
}

// NewSCRAMPBKDF2SHA256 creates a new SCRAMPBKDF2SHA256 instance.
func NewSCRAMPBKDF2SHA256(username, password string) *SCRAMPBKDF2SHA256 {
	return &SCRAMPBKDF2SHA256{username: username, password: password}
}

func (a *SCRAMPBKDF2SHA256) String() string {
	return fmt.Sprintf("method type %s username %s clientChallenge %s clientProof %s salt %s serverChallenge %s rounds %d",
		a.Typ(), trace.Cut(a.username), trace.Redacted(a.clientChallenge), trace.Redacted(a.clientProof), trace.Redacted(a.salt), trace.Redacted(a.serverChallenge), a.rounds)
}

// Compare implements cache.Compare interface.
func (a *SCRAMPBKDF2SHA256) Compare(a1 *SCRAMPBKDF2SHA256) bool {
	return a.password == a1.password && bytes.Equal(a.salt, a1.salt) && a.rounds == a1.rounds
}

// Typ implements the Method interface.
func (a *SCRAMPBKDF2SHA256) Typ() string { return MtSCRAMPBKDF2SHA256 }

// Order implements the Method interface.
func (a *SCRAMPBKDF2SHA256) Order() byte { return MoSCRAMPBKDF2SHA256 }

// AuthLoginName implements the Method interface.
func (a *SCRAMPBKDF2SHA256) AuthLoginName() string { return a.username }

// EncodeInitReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) EncodeInitReq(prms *Prms) error {
	a.clientChallenge = scramClientChallenge()
	prms.addBytes(a.clientChallenge)
	return nil
}

// DecodeInitReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) DecodeInitReq(dec *encoding.Decoder) error {
	_, clientChallenge := dec.LIBytes()
	a.clientChallenge = clientChallenge
	return nil
}

// DecodeInitReply implements the Method interface.
func (a *SCRAMPBKDF2SHA256) DecodeInitReply(dec *encoding.Decoder) error {
	dec.AuthVarFieldInd() // sub parameters
	if err := DecodeAndCheckNumPrm(dec, 3); err != nil {
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
	var err error
	if a.rounds, err = dec.AuthBigUint32(); err != nil {
		return err
	}
	if a.rounds == 0 {
		return fmt.Errorf("invalid PBKDF2 rounds %d", a.rounds)
	}
	return nil
}

// EncodeFinalReq implements the Method interface.
func (a *SCRAMPBKDF2SHA256) EncodeFinalReq(prms *Prms) error {
	key, err := scrampbkdf2KeyCache.Get(a)
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
func (a *SCRAMPBKDF2SHA256) DecodeFinalReq(dec *encoding.Decoder, logonname string) error {
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
func (a *SCRAMPBKDF2SHA256) DecodeFinalReply(dec *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(dec, 2); err != nil {
		return err
	}
	mt := dec.AuthString()
	if err := checkAuthMethodType(mt, a.Typ()); err != nil {
		return err
	}
	dec.AuthVarFieldInd()
	if err := DecodeAndCheckNumPrm(dec, 1); err != nil {
		return err
	}
	serverProof := dec.AuthBytes()
	saltedPassword, err := scrampbkdf2sha256SaltedPassword(a.password, a.salt, int(a.rounds))
	if err != nil {
		return err
	}
	if err := scramVerifyServerProof(saltedPassword, a.salt, a.serverChallenge, a.clientChallenge, serverProof); err != nil {
		return err
	}
	return nil
}
