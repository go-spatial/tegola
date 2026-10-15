// Package compress defines the Compressor interface for go-hdb LZ4 wire
// compression. See the package README for rationale and implementation notes.
package compress

// Compressor provides LZ4 block compression for the HANA wire protocol.
// Implementations are supplied by the application via Connector.SetCompressor.
// A non-nil error from any method aborts the connection.
type Compressor interface {
	EnableWrite() bool                       // whether to compress outbound data (inbound is decompressed when the server compresses it)
	Decompress(src, dst []byte) (int, error) // decompress src into caller-sized dst; returns bytes written
	CompressBound(n int) int                 // safe upper bound (>= n) of compressed size; return 0 (like C LZ4_compressBound) if unable to compute a bound - packet is then sent uncompressed
	Compress(src, dst []byte) (int, error)   // compress src into dst (sized via CompressBound); returns bytes written
}
