//go:build !go1.27

package protocol

import (
	"fmt"
	"io"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
	"golang.org/x/text/transform"
)

// LobScanner is the interface wrapping the Scan method for Lob reading.
type LobScanner interface {
	Scan(w io.Writer) error
}

var _ LobScanner = (*lobOutDescr)(nil)

// LobReader is the interface for reading lob streams.
type LobReader interface {
	ReadLob(request *ReadLobRequest, reply *ReadLobReply) error
}

// lobOutDescr represents a lob output descriptor.
type lobOutDescr struct {
	// if set -> char based
	tr transform.Transformer
	/*
	   readFn is set by decode if additional data packages need to be read (not last data)
	*/
	lobReader LobReader
	chunkSize int
	/*
		HDB does not return lob type code but undefined only
		--> ltc is always ltcUndefined
		--> use isCharBased instead of type code check
	*/
	ltc     lobTypecode
	opt     LobOptions
	numChar int64
	numByte int64
	id      LocatorID
	b       []byte

	// scan attributes.
	wr         io.Writer
	lobRequest *ReadLobRequest
	lobReply   *ReadLobReply
}

// decodeLobOutDescr decodes a lob output descriptor, or returns a real nil for a null value.
// Result is used as driver.Value: a typed nil *lobOutDescr would not be recognized as nil.
func decodeLobOutDescr(dec *encoding.Decoder, tr transform.Transformer, lobReader LobReader, chunkSize int) (any, error) {
	ltc := lobTypecode(dec.Int8())
	opt := LobOptions(dec.Int8())
	if opt.isNull() {
		return nil, nil
	}
	d := &lobOutDescr{tr: tr, lobReader: lobReader, chunkSize: chunkSize, ltc: ltc, opt: opt}
	dec.Skip(2)
	d.numChar = dec.Int64()
	d.numByte = dec.Int64()
	d.id = LocatorID(dec.Uint64())
	size := int(dec.Int32())
	d.b = dec.Bytes(size)
	return d, nil
}

func (d *lobOutDescr) String() string {
	return fmt.Sprintf("typecode %s options %s numChar %d numByte %d id %d bytes %s", d.ltc, d.opt, d.numChar, d.numByte, d.id, trace.Cut(d.b))
}

func (d *lobOutDescr) write(b []byte) (int, error) {
	if d.tr == nil {
		if _, err := d.wr.Write(b); err != nil {
			return len(b), err
		}
		return len(b), nil
	}

	var nDst, numChar int
	var err error
	d.tr.Reset()
	if tr, ok := d.tr.(cesu8.NumCharTransformer); ok { // fasttrack
		nDst, _, numChar, err = tr.TransformNumChar(b, b, false) // cesu8 -> utf8 (always enough space)
	} else { // slow
		nDst, _, err = d.tr.Transform(b, b, false) // cesu8 -> utf8 (always enough space)
		numChar = cesu8.NumChar(b[:nDst])
	}
	if err != nil && err != transform.ErrShortSrc { //nolint: errorlint
		return nDst, err
	}

	if _, err := d.wr.Write(b[:nDst]); err != nil {
		return numChar, err
	}
	return numChar, nil
}

func (d *lobOutDescr) scan(wr io.Writer) error {
	d.wr = wr

	numChar, err := d.write(d.b)
	if err != nil {
		return err
	}

	if d.opt.IsLastData() {
		return nil
	}

	if d.lobRequest == nil {
		d.lobRequest = new(ReadLobRequest)
	}
	if d.lobReply == nil {
		d.lobReply = &ReadLobReply{lobOutDescr: d}
	}
	d.lobRequest.ID = d.id
	d.lobRequest.Ofs = int64(numChar)
	d.lobRequest.ChunkSize = d.chunkSize
	return d.lobReader.ReadLob(d.lobRequest, d.lobReply)
}

// Scan implements the LobScanner interface.
func (d *lobOutDescr) Scan(wr io.Writer) error {
	err := d.scan(wr)
	// if the writer is a pipe-end -> close at the end
	if pwr, ok := wr.(*io.PipeWriter); ok {
		if err != nil {
			pwr.CloseWithError(err)
		} else {
			pwr.Close()
		}
	}
	return err
}

func (d *lobOutDescr) Write() (int, error) {
	n, err := d.write(d.b)
	if err != nil {
		return n, err
	}
	if d.opt.IsLastData() {
		return n, io.EOF
	}
	d.lobRequest.Ofs += int64(n)
	return n, nil
}

// ReadLobReply represents a lob read reply part.
type ReadLobReply struct {
	*lobOutDescr
}

// NewReadLobReply creates a lob read reply part for the given locator id.
func NewReadLobReply(id LocatorID) *ReadLobReply {
	return &ReadLobReply{lobOutDescr: &lobOutDescr{id: id}}
}

func (r *ReadLobReply) String() string {
	return fmt.Sprintf("id %d options %s bytes %s", r.id, r.opt, trace.Cut(r.b))
}

func (r *ReadLobReply) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	if header.numArg() != 1 {
		panic("numArg == 1 expected")
	}
	id := LocatorID(dec.Uint64())
	if id != r.id {
		return fmt.Errorf("invalid locator id %d - expected %d", id, r.id)
	}
	r.opt = LobOptions(dec.Int8())
	size := int(dec.Int32())
	dec.Skip(3)
	r.b = dec.Bytes(size)
	return nil
}
