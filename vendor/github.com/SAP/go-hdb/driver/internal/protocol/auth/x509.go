package auth

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

const (
	x509ServerNonceSize = 64
)

// X509 implements X509 authentication.
type X509 struct {
	certKey     *CertKey
	serverNonce []byte
	logonName   string
}

// NewX509 creates a new X509 instance.
func NewX509(certKey *CertKey) *X509 { return &X509{certKey: certKey} }

func (a *X509) String() string {
	return fmt.Sprintf("method type %s %s serverNonce %s logonName %s",
		a.Typ(), a.certKey, trace.Cut(a.serverNonce), trace.Cut(a.logonName))
}

// Typ implements the Method interface.
func (a *X509) Typ() string { return MtX509 }

// Order implements the Method interface.
func (a *X509) Order() byte { return MoX509 }

// AuthLoginName implements the Method interface.
func (a *X509) AuthLoginName() string { return "" }

// EncodeInitReq implements the Method interface.
func (a *X509) EncodeInitReq(prms *Prms) error {
	// prevent auth call to hdb with invalid certificate
	// as hdb only allows a limited number of unsuccessful authentications
	// - currently only validity period is checked
	if err := a.certKey.validate(time.Now()); err != nil {
		return err
	}
	prms.addEmpty()
	return nil
}

// DecodeInitReq implements the Method interface.
func (a *X509) DecodeInitReq(dec *encoding.Decoder) error {
	_, empty := dec.LIBytes()
	if len(empty) != 0 {
		return errors.New("expected empty parameter")
	}
	return nil
}

// DecodeInitReply implements the Method interface.
func (a *X509) DecodeInitReply(dec *encoding.Decoder) error {
	a.serverNonce = dec.AuthBytes()
	if len(a.serverNonce) != x509ServerNonceSize {
		return fmt.Errorf("invalid server nonce size %d - expected %d", len(a.serverNonce), x509ServerNonceSize)
	}
	return nil
}

// EncodeFinalReq implements the Method interface.
func (a *X509) EncodeFinalReq(prms *Prms) error {
	subPrms := prms.addPrms()

	certBlocks := a.certKey.certBlocks

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

	signature, err := a.certKey.sign(message)
	if err != nil {
		return err
	}
	subPrms.addBytes(signature)
	return nil
}

// DecodeFinalReq implements the Method interface.
func (a *X509) DecodeFinalReq(dec *encoding.Decoder, logonname string) error {
	if logonname != "" {
		return errors.New("expected empty username")
	}
	_, b := dec.LIBytes() // sub parameters
	sub := encoding.NewDecoder(b, nil)
	if err := DecodeAndCheckNumPrm(sub, 3); err != nil {
		return err
	}
	if _, cert := sub.LIBytes(); len(cert) == 0 {
		return errors.New("empty client certificate")
	}
	sub.LIBytes() // chain
	if _, sig := sub.LIBytes(); len(sig) == 0 {
		return errors.New("empty signature")
	}
	return nil
}

// DecodeFinalReply implements the Method interface.
func (a *X509) DecodeFinalReply(dec *encoding.Decoder) error {
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
	var err error
	a.logonName, err = dec.AuthCesu8String()
	return err
}
