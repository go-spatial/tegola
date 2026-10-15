//go:build go1.27

package protocol

// isCharLob returns true if the TypeCode represents a char based lob, false otherwise.
func (tc typeCode) isCharLob() bool {
	return tc == tcText || tc == tcNclob || tc == tcNlocator
}
