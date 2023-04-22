package driver

import (
	"crypto/rand"
)

const (
	// alphanumeric character set
	csAlphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// randAlphanumReader is a global shared instance of an alphanumeric character random generator.
var randAlphanumReader = randAlphanum{}

type randAlphanum struct{}

func (r *randAlphanum) Read(p []byte) (n int, err error) {
	if n, err = rand.Read(p); err != nil {
		return n, err
	}
	size := byte(len(csAlphanum)) // len character sets <= max(byte)
	for i, b := range p {
		p[i] = csAlphanum[b%size]
	}
	return n, nil
}

// randAlphanumString returns a random string of alphanumeric characters and panics if crypto random reader returns an error.
func randAlphanumString(n int) string {
	b := make([]byte, n)
	if _, err := randAlphanumReader.Read(b); err != nil {
		panic(err) // rand should never fail
	}
	return string(b)
}
