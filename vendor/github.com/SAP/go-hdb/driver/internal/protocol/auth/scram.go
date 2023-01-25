package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const (
	clientChallengeSize = 64
	serverChallengeSize = 48
	saltSize            = 16
	clientProofSize     = 32
)

func checkSalt(salt []byte) error {
	if len(salt) != saltSize {
		return fmt.Errorf("invalid salt size %d - expected %d", len(salt), saltSize)
	}
	return nil
}

func checkServerChallenge(serverChallenge []byte) error {
	if len(serverChallenge) != serverChallengeSize {
		return fmt.Errorf("invalid server challenge size %d - expected %d", len(serverChallenge), serverChallengeSize)
	}
	return nil
}

func checkClientProof(clientProof []byte) error {
	if len(clientProof) != clientProofSize {
		return fmt.Errorf("invalid client proof size %d - expected %d", len(clientProof), clientProofSize)
	}
	return nil
}

func clientChallenge() []byte {
	r := make([]byte, clientChallengeSize)
	if _, err := rand.Read(r); err != nil {
		panic(err)
	}
	return r
}

func clientProof(key, salt, serverChallenge, clientChallenge []byte) []byte {
	sig := _hmac(_sha256(key), salt, serverChallenge, clientChallenge)
	proof := xor(sig, key)
	return proof
}

func _sha256(p []byte) []byte {
	hash := sha256.New()
	hash.Write(p)
	s := hash.Sum(nil)
	return s
}

func _hmac(key []byte, prms ...[]byte) []byte {
	hash := hmac.New(sha256.New, key)
	for _, p := range prms {
		hash.Write(p)
	}
	s := hash.Sum(nil)
	return s
}

func xor(sig, key []byte) []byte {
	r := make([]byte, len(sig))

	for i, v := range sig {
		r[i] = v ^ key[i]
	}
	return r
}
