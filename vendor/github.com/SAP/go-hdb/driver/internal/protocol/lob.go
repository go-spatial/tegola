package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/internal/trace"
	"golang.org/x/text/transform"
)

// LobOptions represents a lob option set.
type LobOptions int8

const (
	loNullindicator LobOptions = 0x01
	loDataincluded  LobOptions = 0x02
	loLastdata      LobOptions = 0x04
)

const (
	loNullindicatorText = "null indicator"
	loDataincludedText  = "data included"
	loLastdataText      = "last data"
)

func (o LobOptions) String() string {
	var s []string
	if o&loNullindicator != 0 {
		s = append(s, loNullindicatorText)
	}
	if o&loDataincluded != 0 {
		s = append(s, loDataincludedText)
	}
	if o&loLastdata != 0 {
		s = append(s, loLastdataText)
	}
	return fmt.Sprintf("%v", s)
}

// IsLastData returns true if the last data package was read, false otherwise.
func (o LobOptions) IsLastData() bool { return (o & loLastdata) != 0 }
func (o LobOptions) isNull() bool     { return (o & loNullindicator) != 0 }

// lob typecode.
type lobTypecode int8

const (
	ltcUndefined lobTypecode = 0
	ltcBlob      lobTypecode = 1
	ltcClob      lobTypecode = 2
	ltcNclob     lobTypecode = 3
)

// not used
// type lobFlags bool

// func (f lobFlags) String() string { return fmt.Sprintf("%t", f) }
// func (f *lobFlags) decode(dec *encoding.Decoder, ph *partHeader) error {
// 	*f = lobFlags(dec.Bool())
// 	return dec.Error()
// }
// func (f lobFlags) encode(enc *encoding.Encoder) error { enc.Bool(bool(f)); return nil }

// LobInDescr represents a lob input descriptor.
type LobInDescr struct {
	rd    io.Reader
	cesu8 bool // whether the lob content needs UTF-8 -> CESU-8 transformation
	opt   LobOptions
	pos   int
	buf   bytes.Buffer
	size  int
}

func newLobInDescr(rd io.Reader, cesu8 bool) *LobInDescr {
	return &LobInDescr{rd: rd, cesu8: cesu8}
}

func (d *LobInDescr) String() string {
	return fmt.Sprintf("options %s size %d pos %d bytes %s", d.opt, d.buf.Len(), d.pos, trace.Cut(d.buf.Bytes()))
}

// IsLastData returns true in case of last data package read, false otherwise.
func (d *LobInDescr) IsLastData() bool { return d.opt.IsLastData() }

// FetchFirst fetches the first lob chunk.
//
// It obtains a fresh transformer instance from cesu8EncoderFn (instead of
// reusing a shared one) because lob input readers can interleave: on a bulk
// insert several multi-chunk lobs are live at once and their chunks are pulled
// round-robin. A stateful (custom) transformer shared across them would have
// its state reset/clobbered by one reader while another is mid-transform,
// silently corrupting the data. A fresh instance per reader keeps each one
// isolated. The instance is created only when a transformation is actually
// needed (d.cesu8); the default transformer is stateless, so this costs
// nothing for it.
func (d *LobInDescr) FetchFirst(chunkSize int, cesu8EncoderFn func() transform.Transformer) error {
	if d.cesu8 {
		d.rd = transform.NewReader(d.rd, cesu8EncoderFn())
	}
	return d.fetchNext(chunkSize)
}

// fetchNext fetches the next lob chunk.
func (d *LobInDescr) fetchNext(chunkSize int) error {
	/*
		We need to guarantee, that a max amount of data is read to prevent
		piece wise LOB writing when avoidable
		--> copy up to chunkSize
	*/
	d.buf.Reset()
	_, err := io.CopyN(&d.buf, d.rd, int64(chunkSize))
	d.opt = loDataincluded
	d.size = d.buf.Len()
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	d.opt |= loLastdata
	return nil
}

func (d *LobInDescr) writeFirst(enc *encoding.Encoder) { enc.Bytes(d.buf.Bytes()) }

// LocatorID represents a locator id.
type LocatorID uint64 // byte[locatorIdSize]

/*
write lobs:
- write lob field to the database in chunks
- loop:
  - writeLobRequest
  - writeLobReply
*/

// WriteLobDescr represents a lob descriptor for writes (lob -> db).
type WriteLobDescr struct {
	LobInDescr *LobInDescr
	ID         LocatorID
	opt        LobOptions
	ofs        int64
	b          []byte
}

func (d WriteLobDescr) String() string {
	return fmt.Sprintf("id %d options %s offset %d bytes %s", d.ID, d.opt, d.ofs, trace.Cut(d.b))
}

// IsLastData returns true in case of last data package read, false otherwise.
func (d *WriteLobDescr) IsLastData() bool { return d.opt.IsLastData() }

// FetchNext fetches the next lob chunk.
func (d *WriteLobDescr) FetchNext(chunkSize int) error {
	if err := d.LobInDescr.fetchNext(chunkSize); err != nil {
		return err
	}
	d.opt = d.LobInDescr.opt
	d.ofs = -1 // offset (-1 := append)
	d.b = d.LobInDescr.buf.Bytes()
	return nil
}

// sniffer.
func (d *WriteLobDescr) decode(dec *encoding.Decoder) error {
	d.ID = LocatorID(dec.Uint64())
	d.opt = LobOptions(dec.Int8())
	d.ofs = dec.Int64()
	size := int(dec.Int32())
	d.b = dec.Bytes(size)
	return nil
}

// write chunk to db.
func (d *WriteLobDescr) encode(enc *encoding.Encoder) error {
	enc.Uint64(uint64(d.ID))
	enc.Int8(int8(d.opt))
	enc.Int64(d.ofs)
	enc.Int32(int32(len(d.b))) //nolint: gosec
	enc.Bytes(d.b)
	return nil
}

// WriteLobRequest represents a lob write request part.
type WriteLobRequest struct {
	Descrs []*WriteLobDescr
}

func (r *WriteLobRequest) String() string { return fmt.Sprintf("descriptors %v", r.Descrs) }

func (r *WriteLobRequest) numArg() int { return len(r.Descrs) }

// sniffer.
func (r *WriteLobRequest) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	numArg := header.numArg()

	r.Descrs = make([]*WriteLobDescr, numArg)
	for i := range numArg {
		r.Descrs[i] = &WriteLobDescr{}
		if err := r.Descrs[i].decode(dec); err != nil {
			return err
		}
	}
	return nil
}

func (r *WriteLobRequest) encode(enc *encoding.Encoder) error {
	for _, descr := range r.Descrs {
		if err := descr.encode(enc); err != nil {
			return err
		}
	}
	return nil
}

// WriteLobReply represents a lob write reply part.
type WriteLobReply struct {
	// write lob fields to db (reply)
	// - returns ids which have not been written completely
	IDs []LocatorID
}

func (r *WriteLobReply) String() string { return fmt.Sprintf("ids %v", r.IDs) }

func (r *WriteLobReply) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	numArg := header.numArg()

	r.IDs = slices.Grow(r.IDs, numArg)[:numArg]

	for i := range numArg {
		r.IDs[i] = LocatorID(dec.Uint64())
	}
	return nil
}

// ReadLobRequest represents a lob read request part.
type ReadLobRequest struct {
	/*
	   read lobs:
	   - read lob field from the database in chunks
	   - loop:
	     - readLobRequest
	     - readLobReply

	   - read lob reply
	     seems like readLobReply returns only a result for one lob - even if more than one is requested
	     --> read single lobs
	*/
	ID        LocatorID
	Ofs       int64
	ChunkSize int
}

func (r *ReadLobRequest) String() string {
	return fmt.Sprintf("id %d offset %d size %d", r.ID, r.Ofs, r.ChunkSize)
}

// sniffer.
func (r *ReadLobRequest) decode(dec *encoding.Decoder, _ *PartHeader, _ *ReaderAttrs) error {
	r.ID = LocatorID(dec.Uint64())
	r.Ofs = dec.Int64()
	r.ChunkSize = int(dec.Int32())
	dec.Skip(4)
	return nil
}

func (r *ReadLobRequest) encode(enc *encoding.Encoder) error {
	enc.Uint64(uint64(r.ID))
	enc.Int64(r.Ofs + 1)          // 1-based
	enc.Int32(int32(r.ChunkSize)) //nolint: gosec
	enc.Zeroes(4)
	return nil
}
