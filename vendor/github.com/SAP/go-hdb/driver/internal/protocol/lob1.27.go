//go:build go1.27

package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
)

// LobOutDescr represents a lob output descriptor.
type LobOutDescr struct {
	IsCharLob bool
	/*
		HDB does not return lob type code but undefined only
		--> ltc is always ltcUndefined
		--> use isCharBased instead of type code check
	*/
	ltc     lobTypecode
	Opt     LobOptions
	numChar int64
	numByte int64
	id      LocatorID
	Bytes   []byte
}

func (d *LobOutDescr) String() string {
	return fmt.Sprintf("typecode %s options %s numChar %d numByte %d id %d bytes %s", d.ltc, d.Opt, d.numChar, d.numByte, d.id, trace.Cut(d.Bytes))
}

// decodeLobOutDescr decodes a lob output descriptor, or returns a real nil for a null value.
// Result is used as driver.Value: a typed nil *LobOutDescr would not be recognized as nil.
func decodeLobOutDescr(dec *encoding.Decoder, isCharLob bool) (any, error) {
	ltc := lobTypecode(dec.Int8())
	opt := LobOptions(dec.Int8())
	if opt.isNull() {
		return nil, nil
	}
	d := &LobOutDescr{IsCharLob: isCharLob, ltc: ltc, Opt: opt}
	dec.Skip(2)
	d.numChar = dec.Int64()
	d.numByte = dec.Int64()
	d.id = LocatorID(dec.Uint64())
	size := int(dec.Int32())
	d.Bytes = dec.Bytes(size)
	return d, nil
}

// LocatorID returns the lob locator id.
func (d *LobOutDescr) LocatorID() LocatorID { return d.id }

// ReadLobReply represents a lob read reply part.
type ReadLobReply struct {
	id    LocatorID
	Opt   LobOptions
	Bytes []byte
}

func (r *ReadLobReply) String() string {
	return fmt.Sprintf("id %d options %s bytes %s", r.id, r.Opt, trace.Cut(r.Bytes))
}

// NewReadLobReply creates a lob read reply part for the given locator id.
func NewReadLobReply(id LocatorID) *ReadLobReply {
	return &ReadLobReply{id: id}
}

func (r *ReadLobReply) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	if header.numArg() != 1 {
		panic("numArg == 1 expected")
	}
	id := LocatorID(dec.Uint64())
	if id != r.id {
		return fmt.Errorf("invalid locator id %d - expected %d", id, r.id)
	}
	r.Opt = LobOptions(dec.Int8())
	size := int(dec.Int32())
	dec.Skip(3)
	r.Bytes = dec.Bytes(size)
	return nil
}
