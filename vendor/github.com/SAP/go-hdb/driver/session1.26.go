//go:build !go1.27

package driver

import (
	"context"
	"io"
	"time"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

// readResultPart reads a protocol result part. On Go < 1.27 the lob reader is
// passed through to the protocol decoder.
func readResultPart(ctx context.Context, pi *p.PartInfo, part p.ResultPartDecoder, lobReader p.LobReader) error {
	return pi.ReadResultPart(ctx, part, lobReader)
}

/*
readLob reads output lob or result lob parameters from db.

read lob reply
  - seems like readLobReply returns only a result for one lob - even if more than one is requested
  - --> read single lobs
*/
func (s *session) readLob(ctx context.Context, request *p.ReadLobRequest, reply *p.ReadLobReply) error {
	defer s.metrics.addSQLTimeValue(sqlTimeFetchLob, time.Now())

	var err error
	for err != io.EOF { //nolint: errorlint
		if err = s.pwr.Write(ctx, p.MtWriteLob, false, request); err != nil {
			return err
		}

		for pi, err := range s.prd.Parts(ctx) {
			if err != nil {
				return err
			}
			switch pi.Header.Kind() {
			case p.PkError:
				err = pi.ReadHDBErrors(ctx)
			case p.PkReadLobReply:
				err = pi.ReadPart(ctx, reply)
			default:
				err = pi.SkipPart(ctx)
			}
			if err != nil {
				return err
			}
		}

		_, err = reply.Write()
		if err != nil && err != io.EOF { //nolint: errorlint
			return err
		}
	}
	return nil
}
