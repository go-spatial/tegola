package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const (
	scramClientChallengeSize = 64
	scramServerChallengeSize = 48
	scramSaltSize            = 16
	scramClientProofSize     = 32
)

func scramCheckSalt(salt []byte) error {
	if len(salt) != scramSaltSize {
		return fmt.Errorf("invalid salt size %d - expected %d", len(salt), scramSaltSize)
	}
	return nil
}

func scramCheckServerChallenge(serverChallenge []byte) error {
	if len(serverChallenge) != scramServerChallengeSize {
		return fmt.Errorf("invalid server challenge size %d - expected %d", len(serverChallenge), scramServerChallengeSize)
	}
	return nil
}

func scramClientChallenge() []byte {
	r := make([]byte, scramClientChallengeSize)
	// does not return err starting with go1.24
	rand.Read(r) //nolint: errcheck
	return r
}

func scramClientProof(key, salt, serverChallenge, clientChallenge []byte) ([]byte, error) {
	if len(key) != scramClientProofSize {
		return nil, fmt.Errorf("invalid key size %d - expected %d", len(key), scramClientProofSize)
	}
	sig := scramHMAC(scramSHA256(key), salt, serverChallenge, clientChallenge)
	if len(sig) != scramClientProofSize {
		return nil, fmt.Errorf("invalid sig size %d - expected %d", len(key), scramClientProofSize)
	}
	// xor sig and key into sig (inline: no further allocation).
	for i, v := range key {
		sig[i] ^= v
	}
	return sig, nil
}

func scramSHA256(p []byte) []byte {
	hash := sha256.New()
	hash.Write(p)
	return hash.Sum(nil)
}

func scramHMAC(key []byte, prms ...[]byte) []byte {
	hash := hmac.New(sha256.New, key)
	for _, p := range prms {
		hash.Write(p)
	}
	return hash.Sum(nil)
}
