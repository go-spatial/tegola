package auth

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/SAP/go-hdb/driver/internal/protocol/x509"
)

const (
	x509ServerNonceSize = 64
)

// X509 implements X509 authentication.
type X509 struct {
	certKey     *x509.CertKey
	serverNonce []byte
	logonName   string
}

// NewX509 creates a new authX509 instance.
func NewX509(certKey *x509.CertKey) *X509 { return &X509{certKey: certKey} }

func (a *X509) String() string {
	return fmt.Sprintf("method type %s %s", a.Typ(), a.certKey)
}

// SetCertKey implements the AuthCertKeySetter interface.
func (a *X509) SetCertKey(certKey *x509.CertKey) { a.certKey = certKey }

// Typ implements the CookieGetter interface.
func (a *X509) Typ() string { return MtX509 }

// Order implements the CookieGetter interface.
func (a *X509) Order() byte { return MoX509 }

// PrepareInitReq implements the Method interface.
func (a *X509) PrepareInitReq(prms *Prms) error {
	// prevent auth call to hdb with invalid certificate
	// as hbd only allows a limited number of unsuccessful authentications
	// - currently only validity period is checked
	if err := a.certKey.Validate(time.Now()); err != nil {
		return err
	}
	prms.addString(a.Typ())
	prms.addEmpty()
	return nil
}

// InitRepDecode implements the Method interface.
func (a *X509) InitRepDecode(d *Decoder) error {
	a.serverNonce = d.bytes()
	if len(a.serverNonce) != x509ServerNonceSize {
		return fmt.Errorf("invalid server nonce size %d - expected %d", len(a.serverNonce), x509ServerNonceSize)
	}
	return nil
}

// PrepareFinalReq implements the Method interface.
func (a *X509) PrepareFinalReq(prms *Prms) error {
	prms.addEmpty() // empty username
	prms.addString(a.Typ())

	subPrms := prms.addPrms()

	certBlocks := a.certKey.CertBlocks()

	numBlocks := len(certBlocks)

	message := bytes.NewBuffer(certBlocks[0].Bytes)

	subPrms.addBytes(certBlocks[0].Bytes)

	if numBlocks == 1 {
		subPrms.addEmpty()
	} else {
		chainPrms := subPrms.addPrms()
		for _, block := range certBlocks[1:] {
			message.Write(block.Bytes)
			chainPrms.addBytes(block.Bytes)
		}
	}

	message.Write(a.serverNonce)

	signature, err := sign(a.certKey, message)
	if err != nil {
		return err
	}
	subPrms.addBytes(signature)
	return nil
}

// FinalRepDecode implements the Method interface.
func (a *X509) FinalRepDecode(d *Decoder) error {
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
	var err error
	a.logonName, err = d.cesu8String()
	return err
}

func sign(certKey *x509.CertKey, message *bytes.Buffer) ([]byte, error) {
	signer, err := certKey.Signer()
	if err != nil {
		return nil, err
	}

	hashed := sha256.Sum256(message.Bytes())
	return signer.Sign(rand.Reader, hashed[:], crypto.SHA256)
}
