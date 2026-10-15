//go:build !go1.27

package protocol

import (
	"context"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// PartInfo holds attributes needed on iterating parts.
type PartInfo struct {
	Header         *PartHeader
	Dec            *encoding.Decoder
	ReadHDBErrors  func(context.Context) error
	ReadPart       func(context.Context, PartDecoder) error
	ReadResultPart func(context.Context, ResultPartDecoder, LobReader) error
	SkipPart       func(context.Context) error
}

// ResultPartDecoder represents a protocol result part decoder.
type ResultPartDecoder interface {
	Part
	decodeResult(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs, lobReader LobReader) error
}

// check if result part types implement the result part decoder interface.
var (
	_ ResultPartDecoder = (*OutputParameters)(nil)
	_ ResultPartDecoder = (*Resultset)(nil)
)

func (r *Reader) readResultPart(ctx context.Context, part ResultPartDecoder, lobReader LobReader) (err error) {
	if lobReader == nil {
		panic("missing lob reader") // should never happen
	}
	defer recoverShortBuffer(&err)
	err = part.decodeResult(r.partInfo.Dec, r.partInfo.Header, r.attrs, lobReader)
	if r.protTraceFn != nil {
		r.protTraceFn(ctx, textPar, part)
	}
	return err
}
