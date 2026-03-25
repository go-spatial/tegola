// Package alphanum implements functions for randomized alphanum content.
package alphanum

import (
	"crypto/rand"

	"github.com/SAP/go-hdb/driver/internal/unsafe"
)

const csAlphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // alphanumeric character set.
var numAlphanum = byte(len(csAlphanum))                                             // len character sets <= max(byte)

// Read fills p with random alphanumeric characters and returns the number of read bytes. It never returns an error, and always fills b entirely.
func Read(p []byte) (n int, err error) {
	// starting with go1.24 rand.Read is never returning a error.
	rand.Read(p) //nolint: errcheck
	for i, b := range p {
		p[i] = csAlphanum[b%numAlphanum]
	}
	return n, nil
}

// ReadString returns a random string of alphanumeric characters and panics if crypto random reader returns an error.
func ReadString(n int) string {
	b := make([]byte, n)
	Read(b) //nolint: errcheck
	return unsafe.ByteSlice2String(b)
}
