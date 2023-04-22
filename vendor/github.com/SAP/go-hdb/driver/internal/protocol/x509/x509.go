// Package x509 provides X509 certificate methods.
package x509

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ValidationError is returned in case of X09 certificate validation errors.
type ValidationError struct {
	errorString string
}

func (e ValidationError) Error() string { return e.errorString }

// CertKey represents a X509 certificate and key.
type CertKey struct {
	cert, key  []byte
	certBlocks []*pem.Block
	certs      []*x509.Certificate
	keyBlock   *pem.Block
}

// NewCertKey returns a new certificate and key instance.
func NewCertKey(cert, key []byte) (*CertKey, error) {
	ck := &CertKey{cert: cert, key: key}
	var err error
	if ck.certBlocks, err = decodeClientCert(ck.cert); err != nil {
		return nil, err
	}
	if ck.certs, err = parseCerts(ck.certBlocks); err != nil {
		return nil, err
	}
	if ck.keyBlock, err = decodeClientKey(ck.key); err != nil {
		return nil, err
	}
	return ck, nil
}

func (ck *CertKey) String() string { return fmt.Sprintf("cert %v key %v", ck.cert, ck.key) }

// Equal returns true if the certificate and key equals the instance data, false otherwise.
func (ck *CertKey) Equal(cert, key []byte) bool {
	return bytes.Equal(ck.cert, cert) && bytes.Equal(ck.key, key)
}

// Cert returns the certificate.
func (ck *CertKey) Cert() []byte { return ck.cert }

// Key returns the key.
func (ck *CertKey) Key() []byte { return ck.key }

// CertBlocks returns the PEM blocks of the certificate.
func (ck *CertKey) CertBlocks() []*pem.Block { return ck.certBlocks }

// Validate validates the certificate (currently validity period only).
func (ck *CertKey) Validate(t time.Time) error {
	t = t.UTC() // cert.NotBefore and cert.NotAfter in UTC as well
	for _, cert := range ck.certs {
		// checks
		// .check validity period
		if t.Before(cert.NotBefore) || t.After(cert.NotAfter) {
			issuerRDN := cert.Issuer.ToRDNSequence().String()
			subjectRDN := cert.Subject.ToRDNSequence().String()
			return &ValidationError{fmt.Sprintf("certificate issuer %s subject %s not in validity period from %s to %s - now %s",
				issuerRDN,
				subjectRDN,
				cert.NotBefore,
				cert.NotAfter,
				t,
			)}
		}
	}
	return nil
}

// Signer returns the cryptographic signer of the key.
func (ck *CertKey) Signer() (crypto.Signer, error) {
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

func decodePEM(data []byte) ([]*pem.Block, error) {
	var blocks []*pem.Block
	block, rest := pem.Decode(data)
	for block != nil {
		blocks = append(blocks, block)
		block, rest = pem.Decode(rest)
	}
	return blocks, nil
}

func decodeClientCert(data []byte) ([]*pem.Block, error) {
	blocks, err := decodePEM(data)
	if err != nil {
		return nil, err
	}
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
	blocks, err := decodePEM(data)
	if err != nil {
		return nil, err
	}
	switch {
	case blocks == nil:
		return nil, fmt.Errorf("invalid client key")
	case len(blocks) != 1:
		return nil, fmt.Errorf("invalid number of blocks in key file %d - expected 1", len(blocks))
	}
	block := blocks[0]
	if encryptedBlock(block) {
		return nil, errors.New("client key is password encrypted")
	}
	return block, nil
}
