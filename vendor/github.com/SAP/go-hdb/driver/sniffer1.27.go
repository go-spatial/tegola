//go:build go1.27

package driver

import (
	"context"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

// snifferReadResultPart reads a result part (resultset or output
// parameters). On Go 1.27 the lob reader is not needed (lobs are read via
// session.readLobComplete).
func snifferReadResultPart(ctx context.Context, pi *p.PartInfo, part p.ResultPartDecoder) error {
	return readResultPart(ctx, pi, part, nil)
}
