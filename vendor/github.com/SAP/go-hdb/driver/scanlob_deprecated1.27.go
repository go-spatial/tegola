//go:build go1.27 && !go1.28

package driver

import "io"

// ScanLobBytes deprecated: starting with go1.27 bytes based scan targets are supported natively.
func ScanLobBytes(_ any, _ *[]byte) error { return nil }

// ScanLobString deprecated: starting with go1.27 string based scan targets are supported natively.
func ScanLobString(_ any, _ *string) error { return nil }

// ScanLobWriter deprecated: starting with go1.27 io.Writer based scan targets are supported natively.
func ScanLobWriter(_ any, _ io.Writer) error { return nil }
