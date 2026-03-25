package auth

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
	"unique"
)

// CertValidationError is returned in case of X09 certificate validation errors.
type CertValidationError struct {
	t    time.Time
	cert *x509.Certificate
}

func (e CertValidationError) Error() string {
	return fmt.Sprintf("certificate issuer %s subject %s not in validity period from %s to %s - now %s",
		e.cert.Issuer.ToRDNSequence().String(),
		e.cert.Subject.ToRDNSequence().String(),
		e.cert.NotBefore,
		e.cert.NotAfter,
		e.t,
	)
}

// CertKey represents a X509 certificate and key.
type CertKey struct {
	certHandle, keyHandle unique.Handle[string]
	certBlocks            []*pem.Block
	certs                 []*x509.Certificate
	keyBlock              *pem.Block
}

// NewCertKey returns a new certificate and key instance.
func NewCertKey(certHandle, keyHandle unique.Handle[string]) (*CertKey, error) {
	certBlocks, err := decodeClientCert([]byte(certHandle.Value()))
	if err != nil {
		return nil, err
	}
	certs, err := parseCerts(certBlocks)
	if err != nil {
		return nil, err
	}
	keyBlock, err := decodeClientKey([]byte(keyHandle.Value()))
	if err != nil {
		return nil, err
	}
	return &CertKey{certHandle: certHandle, keyHandle: keyHandle, certBlocks: certBlocks, certs: certs, keyBlock: keyBlock}, nil
}

func (ck *CertKey) String() string {
	return fmt.Sprintf("cert %s key %s", ck.certHandle.Value(), ck.keyHandle.Value())
}

// Equal returns true if the certificate and key equals the instance data, false otherwise.
func (ck *CertKey) Equal(certHandle, keyHandle unique.Handle[string]) bool {
	return certHandle == ck.certHandle && keyHandle == ck.keyHandle
}

// Cert returns the certificate.
func (ck *CertKey) Cert() []byte { return []byte(ck.certHandle.Value()) }

// Key returns the key.
func (ck *CertKey) Key() []byte { return []byte(ck.keyHandle.Value()) }

// validate validates the certificate (currently validity period only).
func (ck *CertKey) validate(t time.Time) error { return validateCerts(ck.certs, t) }

// validate validates the certificate (currently validity period only).
func validateCerts(certs []*x509.Certificate, t time.Time) error {
	t = t.UTC() // cert.NotBefore and cert.NotAfter in UTC as well
	for _, cert := range certs {
		// checks
		// .check validity period
		if t.Before(cert.NotBefore) || t.After(cert.NotAfter) {
			return &CertValidationError{t: t, cert: cert}
		}
	}
	return nil
}

// signer returns the cryptographic signer of the key.
func (ck *CertKey) signer() (crypto.Signer, error) {
	switch ck.keyBlock.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(ck.keyBlock.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(ck.keyBlock.Bytes)
		if err != nil {
			return nil, err
		}
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, errors.New("internal error: parsed PKCS8 private key is not a crypto.Signer")
		}
		return signer, nil
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(ck.keyBlock.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key type %q", ck.keyBlock.Type)
	}
}

func ecdsaDigest(publicKey *ecdsa.PublicKey, message *bytes.Buffer) ([]byte, crypto.Hash) {
	switch {
	case publicKey.Params().BitSize <= 256:
		hashed := sha256.Sum256(message.Bytes())
		return hashed[:], crypto.SHA256
	case publicKey.Params().BitSize <= 384:
		hashed := sha512.Sum384(message.Bytes())
		return hashed[:], crypto.SHA384
	default:
		hashed := sha512.Sum512(message.Bytes())
		return hashed[:], crypto.SHA512
	}
}

func digest(pubkey crypto.PublicKey, message *bytes.Buffer) ([]byte, crypto.Hash, error) {
	switch pubkey := pubkey.(type) {
	case rsa.PublicKey, *rsa.PublicKey:
		hashed := sha256.Sum256(message.Bytes())
		return hashed[:], crypto.SHA256, nil
	case ecdsa.PublicKey:
		b, hash := ecdsaDigest(&pubkey, message)
		return b, hash, nil
	case *ecdsa.PublicKey:
		b, hash := ecdsaDigest(pubkey, message)
		return b, hash, nil
	case ed25519.PublicKey, *ed25519.PublicKey:
		// hashing is done by the signer
		return message.Bytes(), 0, nil
	default:
		return nil, 0, fmt.Errorf("unsupported key type for signing")
	}
}

func (ck *CertKey) sign(message *bytes.Buffer) ([]byte, error) {
	signer, err := ck.signer()
	if err != nil {
		return nil, err
	}

	digest, hash, err := digest(signer.Public(), message)
	if err != nil {
		return nil, err
	}

	return signer.Sign(rand.Reader, digest, hash)
}

func decodePEM(data []byte) []*pem.Block {
	var blocks []*pem.Block
	block, rest := pem.Decode(data)
	for block != nil {
		blocks = append(blocks, block)
		block, rest = pem.Decode(rest)
	}
	return blocks
}

func decodeClientCert(data []byte) ([]*pem.Block, error) {
	blocks := decodePEM(data)
	switch {
	case blocks == nil:
		return nil, errors.New("invalid client certificate")
	case len(blocks) < 1:
		return nil, fmt.Errorf("invalid number of blocks in certificate file %d - expected min 1", len(blocks))
	}
	return blocks, nil
}

func parseCerts(blocks []*pem.Block) (certs []*x509.Certificate, err error) {
	for _, block := range blocks {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// encryptedBlock tells whether a private key is
// encrypted by examining its Proc-Type header
// for a mention of ENCRYPTED
// according to RFC 1421 Section 4.6.1.1.
func encryptedBlock(block *pem.Block) bool {
	return strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED")
}

func decodeClientKey(data []byte) (*pem.Block, error) {
	blocks := decodePEM(data)
	switch {
	case blocks == nil:
		return nil, errors.New("invalid client key")
	case len(blocks) != 1:
		return nil, fmt.Errorf("invalid number of blocks in key file %d - expected 1", len(blocks))
	}
	block := blocks[0]
	if encryptedBlock(block) {
		return nil, errors.New("client key is password encrypted")
	}
	return block, nil
}
