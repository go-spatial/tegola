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
	"errors"
	"fmt"
	"strings"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

const (
	ldapClientChallengeSize = 64
	ldapServerChallengeSize = 64
	ldapCapabilitiesSize    = 8
	ldapDefaultCapabilities = 0x01 // called "default capabilities" in node-hdb
	ldapSessionKeySize      = 32   // AES-256 key size
	ldapPublicKeyBitSize    = 2048 // required server RSA key size
	ldapPublicKeyBufLen     = 580  // maximum server RSA public key length in bytes
)

// LDAP implements LDAP authentication.
type LDAP struct {
	username            string
	password            string
	clientChallenge     []byte
	serverChallenge     []byte
	serverPublicKey     *rsa.PublicKey
	encryptedSessionKey []byte
	encryptedPassword   []byte
}

// NewLDAP creates a new LDAP authentication instance.
func NewLDAP(username, password string) *LDAP {
	return &LDAP{
		username: username,
		password: password,
	}
}

func (a *LDAP) String() string {
	b := &strings.Builder{}
	fmt.Fprintf(b,
		"method type %s username %s clientChallenge %s serverChallenge %s encryptedSessionKey %s encryptedPassword %s",
		a.Typ(), trace.Cut(a.username), trace.Cut(a.clientChallenge), trace.Cut(a.serverChallenge),
		trace.Cut(a.encryptedSessionKey), trace.Cut(a.encryptedPassword))
	if a.serverPublicKey != nil {
		if der, err := x509.MarshalPKIXPublicKey(a.serverPublicKey); err == nil {
			fmt.Fprintf(b, " serverPublicKey %s", trace.Cut(der))
		}
	}
	return b.String()
}

// Typ implements the Method interface.
func (a *LDAP) Typ() string { return MtLDAP }

// Order implements the Method interface.
func (a *LDAP) Order() byte { return MoLDAP }

// AuthLoginName implements the Method interface.
func (a *LDAP) AuthLoginName() string { return a.username }

// EncodeInitReq implements the Method interface.
func (a *LDAP) EncodeInitReq(prms *Prms) error {
	a.clientChallenge = make([]byte, ldapClientChallengeSize)
	rand.Read(a.clientChallenge)

	// Add sub-parameters: client challenge and capabilities
	subPrms := prms.addPrms()
	subPrms.addBytes(a.clientChallenge)

	capabilities := make([]byte, ldapCapabilitiesSize)
	capabilities[0] = ldapDefaultCapabilities
	subPrms.addBytes(capabilities)

	return nil
}

// DecodeInitReq implements the Method interface.
func (a *LDAP) DecodeInitReq(dec *encoding.Decoder) error {
	_, b := dec.LIBytes() // sub parameters
	sub := encoding.NewDecoder(b, nil)
	if err := DecodeAndCheckNumPrm(sub, 2); err != nil {
		return err
	}

	_, clientChallenge := sub.LIBytes()
	if len(clientChallenge) != ldapClientChallengeSize {
		return fmt.Errorf("invalid client challenge size %d - expected %d", len(clientChallenge), ldapClientChallengeSize)
	}
	a.clientChallenge = clientChallenge

	_, capabilities := sub.LIBytes()
	if len(capabilities) != ldapCapabilitiesSize {
		return fmt.Errorf("invalid capabilities size %d - expected %d", len(capabilities), ldapCapabilitiesSize)
	}
	return nil
}

// DecodeInitReply implements the Method interface.
func (a *LDAP) DecodeInitReply(dec *encoding.Decoder) error {
	dec.AuthVarFieldInd()
	if err := DecodeAndCheckNumPrm(dec, 4); err != nil {
		return fmt.Errorf("LDAP authentication: %w", err)
	}

	clientChallenge := dec.AuthBytes()
	if len(clientChallenge) != ldapClientChallengeSize {
		return fmt.Errorf("invalid client challenge size %d - expected %d", len(clientChallenge), ldapClientChallengeSize)
	}

	a.serverChallenge = dec.AuthBytes()
	if len(a.serverChallenge) != ldapServerChallengeSize {
		return fmt.Errorf("invalid server challenge size %d - expected %d", len(a.serverChallenge), ldapServerChallengeSize)
	}

	serverPublicKeyPEM := dec.AuthBytes()
	if len(serverPublicKeyPEM) == 0 {
		return errors.New("server did not provide RSA public key")
	}
	if len(serverPublicKeyPEM) > ldapPublicKeyBufLen {
		return fmt.Errorf("invalid server RSA public key length %d - maximum %d", len(serverPublicKeyPEM), ldapPublicKeyBufLen)
	}

	capabilities := dec.AuthBytes()
	if len(capabilities) == 0 {
		return errors.New("empty server capabilities")
	}
	if capabilities[0] != ldapDefaultCapabilities {
		return fmt.Errorf("unknown server capabilities %x", capabilities)
	}

	var err error
	a.serverPublicKey, err = ldapParseRSAPublicKey(serverPublicKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse server public key: %w", err)
	}
	if a.serverPublicKey.N.BitLen() != ldapPublicKeyBitSize {
		return fmt.Errorf("invalid server RSA public key size %d bits - expected %d", a.serverPublicKey.N.BitLen(), ldapPublicKeyBitSize)
	}

	if !bytes.Equal(clientChallenge, a.clientChallenge) {
		return fmt.Errorf("%w: LDAP authentication: client challenge mismatch", ErrAuthVerifyFailed)
	}
	return nil
}

// EncodeFinalReq implements the Method interface.
func (a *LDAP) EncodeFinalReq(prms *Prms) error {
	// Generate random session key
	sessionKey := make([]byte, ldapSessionKeySize)
	rand.Read(sessionKey)

	encryptedSessionKey, err := ldapEncryptSessionKey(sessionKey, a.serverChallenge, a.serverPublicKey)
	if err != nil {
		return err
	}

	encryptedPassword, err := ldapEncryptPassword(a.password, sessionKey, a.serverChallenge)
	if err != nil {
		return err
	}

	subPrms := prms.addPrms()
	subPrms.addBytes(encryptedSessionKey)
	subPrms.addBytes(encryptedPassword)

	return nil
}

// DecodeFinalReq implements the Method interface.
func (a *LDAP) DecodeFinalReq(dec *encoding.Decoder, logonname string) error {
	a.username = logonname
	_, b := dec.LIBytes() // sub parameters
	sub := encoding.NewDecoder(b, nil)
	if err := DecodeAndCheckNumPrm(sub, 2); err != nil {
		return err
	}
	_, a.encryptedSessionKey = sub.LIBytes()
	_, a.encryptedPassword = sub.LIBytes()
	return nil
}

// DecodeFinalReply implements the Method interface.
func (a *LDAP) DecodeFinalReply(dec *encoding.Decoder) error {
	if err := DecodeAndCheckNumPrm(dec, 2); err != nil {
		return fmt.Errorf("LDAP authentication: %w", err)
	}

	methodName := dec.AuthString()
	if err := checkAuthMethodType(methodName, a.Typ()); err != nil {
		return err
	}
	serverProof := dec.AuthBytes()
	if len(serverProof) > 0 {
		return fmt.Errorf("server proof failed: %v", serverProof)
	}
	return nil
}

func ldapParseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM data")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
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
	padtext := bytes.Repeat([]byte{byte(padding)}, padding) //nolint: gosec
	return append(data, padtext...)
}
