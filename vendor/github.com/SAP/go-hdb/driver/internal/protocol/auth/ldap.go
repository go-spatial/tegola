// SPDX-FileCopyrightText: 2014-2024 SAP SE
//
// SPDX-License-Identifier: Apache-2.0

// Implementation details are based on
// https://github.com/SAP/node-hdb/blob/master/lib/protocol/auth/LDAP.js

package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint: gosec //
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

const (
	ldapClientChallengeSize = 64
	ldapServerChallengeSize = 64
	ldapCapabilitiesSize    = 8
	ldapDefaultCapabilities = 0x01 // called "default capabilities" in node-hdb
	ldapSessionKeySize      = 32   // AES-256 key size
)

// LDAP implements LDAP authentication.
type LDAP struct {
	username        string
	password        string
	clientChallenge []byte
	serverChallenge []byte
	serverPublicKey *rsa.PublicKey
}

// NewLDAP creates a new LDAP authentication instance.
func NewLDAP(username, password string) *LDAP {
	return &LDAP{
		username: username,
		password: password,
	}
}

func (a *LDAP) String() string {
	return fmt.Sprintf("method type %s username %s", a.Typ(), a.username)
}

// Typ implements the Method interface.
func (a *LDAP) Typ() string { return MtLDAP }

// Order implements the Method interface.
func (a *LDAP) Order() byte { return MoLDAP }

// PrepareInitReq implements the Method interface.
func (a *LDAP) PrepareInitReq(prms *Prms) error {
	a.clientChallenge = make([]byte, ldapClientChallengeSize)
	rand.Read(a.clientChallenge) //nolint: errcheck // never returns error

	prms.addString(a.Typ())

	// Add sub-parameters: client challenge and capabilities
	subPrms := prms.addPrms()
	subPrms.addBytes(a.clientChallenge)

	capabilities := make([]byte, ldapCapabilitiesSize)
	capabilities[0] = ldapDefaultCapabilities
	subPrms.addBytes(capabilities)

	return nil
}

// InitRepDecode implements the Method interface.
func (a *LDAP) InitRepDecode(d *encoding.Decoder) error {
	d.AuthVarFieldInd()
	if err := DecodeAndCheckNumPrm(d, 4); err != nil {
		return fmt.Errorf("LDAP authentication: %w", err)
	}

	clientChallenge := d.AuthBytes()
	if len(clientChallenge) != ldapClientChallengeSize {
		return fmt.Errorf("invalid client challenge size %d - expected %d", len(clientChallenge), ldapClientChallengeSize)
	}

	a.serverChallenge = d.AuthBytes()
	if len(a.serverChallenge) != ldapServerChallengeSize {
		return fmt.Errorf("invalid server challenge size %d - expected %d", len(a.serverChallenge), ldapServerChallengeSize)
	}

	serverPublicKeyPEM := d.AuthBytes()
	if len(serverPublicKeyPEM) == 0 {
		return fmt.Errorf("server did not provide RSA public key")
	}

	capabilities := d.AuthBytes()
	if len(capabilities) == 0 {
		return fmt.Errorf("empty server capabilities")
	}
	if capabilities[0] != ldapDefaultCapabilities {
		return fmt.Errorf("unknown server capabilities %x", capabilities)
	}

	var err error
	a.serverPublicKey, err = ldapParseRSAPublicKey(serverPublicKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse server public key: %w", err)
	}

	if !bytes.Equal(clientChallenge, a.clientChallenge) {
		return fmt.Errorf("LDAP authentication: client challenge mismatch")
	}
	return nil
}

// PrepareFinalReq implements the Method interface.
func (a *LDAP) PrepareFinalReq(prms *Prms) error {
	// Generate random session key
	sessionKey := make([]byte, ldapSessionKeySize)
	rand.Read(sessionKey) //nolint:errcheck

	encryptedSessionKey, err := ldapEncryptSessionKey(sessionKey, a.serverChallenge, a.serverPublicKey)
	if err != nil {
		return err
	}

	encryptedPassword, err := ldapEncryptPassword(a.password, sessionKey, a.serverChallenge)
	if err != nil {
		return err
	}

	prms.AddCESU8String(a.username)
	prms.addString(a.Typ())

	subPrms := prms.addPrms()
	subPrms.addBytes(encryptedSessionKey)
	subPrms.addBytes(encryptedPassword)

	return nil
}

// FinalRepDecode implements the Method interface.
func (a *LDAP) FinalRepDecode(d *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(d, 2); err != nil {
		return fmt.Errorf("LDAP authentication: %w", err)
	}

	methodName := d.AuthString()
	if err := checkAuthMethodType(methodName, a.Typ()); err != nil {
		return err
	}
	serverProof := d.AuthBytes()
	if len(serverProof) > 0 {
		return fmt.Errorf("server proof failed: %v", serverProof)
	}
	return nil
}

func ldapParseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

func ldapEncryptSessionKey(sessionKey []byte, challenge []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	plaintext := make([]byte, len(sessionKey)+len(challenge))
	copy(plaintext[:len(sessionKey)], sessionKey)
	copy(plaintext[len(sessionKey):], challenge)

	ciphertext, err := rsa.EncryptOAEP(
		sha1.New(), //nolint:gosec
		rand.Reader,
		publicKey,
		plaintext,
		nil, // no label
	)
	if err != nil {
		return nil, err
	}

	return ciphertext, nil
}

func ldapEncryptPassword(password string, sessionKey []byte, challenge []byte) ([]byte, error) {
	passwordBytes := []byte(password)

	plaintext := make([]byte, len(passwordBytes)+1+len(challenge))
	copy(plaintext, passwordBytes)
	plaintext[len(passwordBytes)] = 0x00 // Magic separator byte
	copy(plaintext[len(passwordBytes)+1:], challenge)

	plaintext = ldapPKCS7Pad(plaintext, aes.BlockSize)

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	iv := challenge[:aes.BlockSize]

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

// ldapPKCS7Pad pads data to a multiple of blockSize using PKCS7 padding.
func ldapPKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}
