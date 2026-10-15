//go:build liblz4

package compress

// liblz4 is build-tagged so neither cgo nor liblz4 are dependencies of
// the default build path or of go-hdb consumers.
//
// developer-machine prerequisites:
// .macOS: brew install lz4
// .Linux: apt install liblz4-dev (or distro equivalent)
//
// run: go test -v -tags=liblz4

// #cgo pkg-config: liblz4
// #include <lz4.h>
import "C"

import (
	"errors"
	"unsafe"
)

var (
	errCompress   = errors.New("LZ4_compress_default failed")
	errDecompress = errors.New("LZ4_decompress_safe failed")
)

// liblz4CompressBound mirrors LZ4_compressBound — max bytes the C encoder may
// write for an input of the given size.
func liblz4CompressBound(n int) int {
	return int(C.LZ4_compressBound(C.int(n)))
}

// liblz4Compress wraps LZ4_compress_default; returns bytes written or an error
// on failure (caller is responsible for sizing dst via cCompressBound).
func liblz4Compress(src, dst []byte) (int, error) {
	if len(src) == 0 {
		return 0, nil
	}
	n := int(C.LZ4_compress_default(
		(*C.char)(unsafe.Pointer(&src[0])),
		(*C.char)(unsafe.Pointer(&dst[0])),
		C.int(len(src)),
		C.int(len(dst)),
	))
	if n <= 0 {
		return n, errCompress
	}
	return n, nil
}

// liblz4Decompress wraps LZ4_decompress_safe; returns bytes written or an error.
func liblz4Decompress(src, dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	n := int(C.LZ4_decompress_safe(
		(*C.char)(unsafe.Pointer(&src[0])),
		(*C.char)(unsafe.Pointer(&dst[0])),
		C.int(len(src)),
		C.int(len(dst)),
	))
	if n < 0 {
		return 0, errDecompress
	}
	return n, nil
}
