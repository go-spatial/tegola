//go:build !liblz4

package compress

// DefaultCompressor is the default lz4 compression implementation.
var DefaultCompressor Compressor = nil // no lz4 compression
