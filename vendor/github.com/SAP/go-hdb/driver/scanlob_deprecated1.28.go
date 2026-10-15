//go:build go1.28

package driver

import "io"

// ScanLobBytes deprecated: fully deprecated as of go1.28 - panics if called.
// Starting with go1.27 bytes based scan targets are supported natively.
func ScanLobBytes(_ any, _ *[]byte) error {
	panic("driver.ScanLobBytes: deprecated as of go1.28 - scan lob values natively into []byte")
}

// ScanLobString deprecated: fully deprecated as of go1.28 - panics if called.
// Starting with go1.27 string based scan targets are supported natively.
func ScanLobString(_ any, _ *string) error {
	panic("driver.ScanLobString: deprecated as of go1.28 - scan lob values natively into string")
}

// ScanLobWriter deprecated: fully deprecated as of go1.28 - panics if called.
// Starting with go1.27 io.Writer based scan targets are supported natively.
func ScanLobWriter(_ any, _ io.Writer) error {
	panic("driver.ScanLobWriter: deprecated as of go1.28 - scan lob values natively into io.Writer")
}
