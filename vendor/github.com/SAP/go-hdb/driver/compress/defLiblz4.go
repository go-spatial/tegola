//go:build liblz4

package compress

type liblz4 struct{}

func (liblz4) EnableWrite() bool { return true }

func (liblz4) Decompress(src, dst []byte) (int, error) {
	return liblz4Decompress(src, dst)
}

func (liblz4) CompressBound(n int) int {
	return liblz4CompressBound(n)
}

func (liblz4) Compress(src, dst []byte) (int, error) {
	return liblz4Compress(src, dst)
}

var DefaultCompressor Compressor = liblz4{}
