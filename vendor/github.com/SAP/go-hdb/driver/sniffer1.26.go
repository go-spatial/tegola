//go:build !go1.27

package driver

import (
	"context"
	"io"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

// noopLobReader is a placeholder that satisfies the go1.26 non-nil lob reader
// requirement. It is never invoked during trace decoding (readLob is only
// called on scan), so it just returns io.EOF.
type noopLobReader struct{}

func (noopLobReader) ReadLob(*p.ReadLobRequest, *p.ReadLobReply) error { return io.EOF }

// snifferReadResultPart reads a result part (resultset or output
// parameters). On Go < 1.27 the decoder requires a non-nil lob reader, which
// is supplied by noopLobReader.
func snifferReadResultPart(ctx context.Context, pi *p.PartInfo, part p.ResultPartDecoder) error {
	return readResultPart(ctx, pi, part, noopLobReader{})
}
