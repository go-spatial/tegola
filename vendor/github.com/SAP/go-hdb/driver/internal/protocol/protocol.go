package protocol

import (
	"bufio"
	"fmt"
	"io"
	"math"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/sqltrace"
	"golang.org/x/text/transform"
)

// padding
const padding = 8

func padBytes(size int) int {
	if r := size % padding; r != 0 {
		return padding - r
	}
	return 0
}

// Reader represents a protocol reader.
type Reader struct {
	upStream bool
	tracer   func(up bool, v any) // performance
	traceOn  bool

	step int // authentication

	dec *encoding.Decoder

	mh *messageHeader
	sh *segmentHeader
	ph *PartHeader

	readBytes int64
	numPart   int
	cntPart   int
	partRead  bool

	partReaderCache map[PartKind]partReader

	lastErrors       *HdbErrors
	lastRowsAffected *RowsAffected

	// partReader read errors could be
	// - read buffer errors -> buffer Error() and ResetError()
	// - plus other errors (which cannot be ignored, e.g. Lob reader)
	err error
}

// NewReader returns an instance of a protocol reader.
func NewReader(upStream bool, rd io.Reader, decoder func() transform.Transformer) *Reader {
	tracer, on := newTracer()
	return &Reader{
		upStream:        upStream,
		tracer:          tracer,
		traceOn:         on,
		dec:             encoding.NewDecoder(rd, decoder),
		partReaderCache: map[PartKind]partReader{},
		mh:              &messageHeader{},
		sh:              &segmentHeader{},
		ph:              &PartHeader{},
	}
}

// SetDfv sets the data format version fpr the protocol reader.
func (r *Reader) SetDfv(dfv int) { r.dec.SetDfv(dfv) }

// ReadSkip reads the server reply without returning the results.
func (r *Reader) ReadSkip() error { return r.IterateParts(nil) }

// SessionID returns the message header session id.
func (r *Reader) SessionID() int64 { return r.mh.sessionID }

// FunctionCode returns the segment header function code.
func (r *Reader) FunctionCode() FunctionCode { return r.sh.functionCode }

func (r *Reader) readInitRequest() error {
	req := &initRequest{}
	if err := req.decode(r.dec); err != nil {
		return err
	}
	r.tracer(r.upStream, req)
	return nil
}

func (r *Reader) readInitReply() error {
	rep := &initReply{}
	if err := rep.decode(r.dec); err != nil {
		return err
	}
	r.tracer(r.upStream, rep)
	return nil
}

// ReadProlog reads the protocol prolog.
func (r *Reader) ReadProlog() error {
	if r.upStream {
		return r.readInitRequest()
	}
	return r.readInitReply()
}

func (r *Reader) checkError() error {
	defer func() { // init readFlags
		r.lastErrors = nil
		r.lastRowsAffected = nil
		r.err = nil
		r.dec.ResetError()
	}()

	if r.err != nil {
		return r.err
	}

	if err := r.dec.Error(); err != nil {
		return err
	}

	if r.lastErrors == nil {
		return nil
	}

	if r.lastRowsAffected != nil { // link statement to error
		j := 0
		for i, rows := range *r.lastRowsAffected {
			if rows == RaExecutionFailed {
				r.lastErrors.SetStmtNo(j, i)
				j++
			}
		}
	}

	if r.lastErrors.HasWarnings() {
		r.lastErrors.ErrorsFunc(func(err error) {
			sqltrace.Trace.Println(err)
		})
		return nil
	}

	return r.lastErrors
}

func (r *Reader) Read(part partReader) error {
	r.partRead = true

	err := r.readPart(part)
	if err != nil {
		r.err = err
	}

	switch part := part.(type) {
	case *HdbErrors:
		r.lastErrors = part
	case *RowsAffected:
		r.lastRowsAffected = part
	}
	return err
}

func (r *Reader) authPart() partReader {
	defer func() { r.step++ }()

	switch {
	case r.upStream && r.step == 0:
		return &AuthInitRequest{}
	case r.upStream:
		return &AuthFinalRequest{}
	case !r.upStream && r.step == 0:
		return &AuthInitReply{}
	case !r.upStream:
		return &AuthFinalReply{}
	default:
		panic(fmt.Errorf("invalid auth step in protocol reader %d", r.step))
	}
}

func (r *Reader) skip() error {
	pk := r.ph.PartKind

	// if trace is on or mandatory parts need to be read we cannot skip
	if !(r.traceOn || pk == PkError || pk == PkRowsAffected) {
		return r.skipPart()
	}

	if pk == PkAuthentication {
		return r.Read(r.authPart())
	}

	// check part cache
	if part, ok := r.partReaderCache[pk]; ok {
		return r.Read(part)
	}

	part, ok := newGenPartReader(pk)
	if !ok { // part is not yet supported -> skip
		return r.skipPart()
	}

	// cache part
	r.partReaderCache[pk] = part

	return r.Read(part)
}

func (r *Reader) skipPadding() int64 {
	if r.cntPart != r.numPart { // padding if not last part
		padBytes := padBytes(int(r.ph.bufferLength))
		r.dec.Skip(padBytes)
		return int64(padBytes)
	}

	// last part:
	// skip difference between real read bytes and message header var part length
	padBytes := int64(r.mh.varPartLength) - r.readBytes
	switch {
	case padBytes < 0:
		panic(fmt.Errorf("protocol error: bytes read %d > variable part length %d", r.readBytes, r.mh.varPartLength))
	case padBytes > 0:
		r.dec.Skip(int(padBytes))
	}
	return padBytes
}

func (r *Reader) skipPart() error {
	r.dec.ResetCnt()
	r.dec.Skip(int(r.ph.bufferLength))
	r.tracer(r.upStream, "*skipped")

	r.readBytes += int64(r.dec.Cnt())
	r.readBytes += r.skipPadding()
	return nil
}

func (r *Reader) readPart(part partReader) error {
	r.dec.ResetCnt()
	err := part.decode(r.dec, r.ph) // do not return here in case of error -> read stream would be broken
	cnt := r.dec.Cnt()
	r.tracer(r.upStream, part)

	bufferLen := int(r.ph.bufferLength)
	switch {
	case cnt < bufferLen: // protocol buffer length > read bytes -> skip the unread bytes
		r.dec.Skip(bufferLen - cnt)
	case cnt > bufferLen: // read bytes > protocol buffer length -> should never happen
		panic(fmt.Errorf("protocol error: read bytes %d > buffer length %d", cnt, bufferLen))
	}

	r.readBytes += int64(r.dec.Cnt())
	r.readBytes += r.skipPadding()
	return err
}

// IterateParts is iterating over the parts returned by the server.
func (r *Reader) IterateParts(partFn func(ph *PartHeader)) error {
	if err := r.mh.decode(r.dec); err != nil {
		return err
	}
	r.readBytes = 0 // header bytes are not calculated in header varPartBytes: start with zero

	r.tracer(r.upStream, r.mh)

	for i := 0; i < int(r.mh.noOfSegm); i++ {
		if err := r.sh.decode(r.dec); err != nil {
			return err
		}

		r.readBytes += segmentHeaderSize

		r.tracer(r.upStream, r.sh)

		r.numPart = int(r.sh.noOfParts)
		r.cntPart = 0

		for j := 0; j < int(r.sh.noOfParts); j++ {

			if err := r.ph.decode(r.dec); err != nil {
				return err
			}

			r.readBytes += partHeaderSize

			r.tracer(r.upStream, r.ph)

			r.cntPart++

			r.partRead = false
			if partFn != nil {
				partFn(r.ph)
			}
			if !r.partRead {
				r.skip()
			}
		}
	}
	return r.checkError()
}

// Writer represents a protocol writer.
type Writer struct {
	tracer func(up bool, v any) // performance

	wr  *bufio.Writer
	enc *encoding.Encoder

	sv     map[string]string
	svSent bool

	// reuse header
	mh *messageHeader
	sh *segmentHeader
	ph *PartHeader
}

// NewWriter returns an instance of a protocol writer.
func NewWriter(wr *bufio.Writer, encoder func() transform.Transformer, sv map[string]string) *Writer {
	tracer, _ := newTracer()
	return &Writer{
		tracer: tracer,
		wr:     wr,
		sv:     sv,
		enc:    encoding.NewEncoder(wr, encoder),
		mh:     new(messageHeader),
		sh:     new(segmentHeader),
		ph:     new(PartHeader),
	}
}

const (
	productVersionMajor  = 4
	productVersionMinor  = 20
	protocolVersionMajor = 4
	protocolVersionMinor = 1
)

// WriteProlog writes the protocol prolog.
func (w *Writer) WriteProlog() error {
	req := &initRequest{}
	req.product.major = productVersionMajor
	req.product.minor = productVersionMinor
	req.protocol.major = protocolVersionMajor
	req.protocol.minor = protocolVersionMinor
	req.numOptions = 1
	req.endianess = littleEndian
	if err := req.encode(w.enc); err != nil {
		return err
	}
	w.tracer(true, req)
	return w.wr.Flush()
}

func (w *Writer) Write(sessionID int64, messageType MessageType, commit bool, writers ...partWriter) error {
	// check on session variables to be send as ClientInfo
	if w.sv != nil && !w.svSent && messageType.ClientInfoSupported() {
		writers = append([]partWriter{clientInfo(w.sv)}, writers...)
		w.svSent = true
	}

	numWriters := len(writers)
	partSize := make([]int, numWriters)
	size := int64(segmentHeaderSize + numWriters*partHeaderSize) //int64 to hold MaxUInt32 in 32bit OS

	for i, part := range writers {
		s := part.size()
		size += int64(s + padBytes(s))
		partSize[i] = s // buffer size (expensive calculation)
	}

	if size > math.MaxUint32 {
		return fmt.Errorf("message size %d exceeds maximum message header value %d", size, int64(math.MaxUint32)) //int64: without cast overflow error in 32bit OS
	}

	bufferSize := size

	w.mh.sessionID = sessionID
	w.mh.varPartLength = uint32(size)
	w.mh.varPartSize = uint32(bufferSize)
	w.mh.noOfSegm = 1

	if err := w.mh.encode(w.enc); err != nil {
		return err
	}
	w.tracer(true, w.mh)

	if size > math.MaxInt32 {
		return fmt.Errorf("message size %d exceeds maximum part header value %d", size, math.MaxInt32)
	}

	w.sh.messageType = messageType
	w.sh.commit = commit
	w.sh.segmentKind = skRequest
	w.sh.segmentLength = int32(size)
	w.sh.segmentOfs = 0
	w.sh.noOfParts = int16(numWriters)
	w.sh.segmentNo = 1

	if err := w.sh.encode(w.enc); err != nil {
		return err
	}
	w.tracer(true, w.sh)

	bufferSize -= segmentHeaderSize

	for i, part := range writers {

		size := partSize[i]
		pad := padBytes(size)

		w.ph.PartKind = part.kind()
		if err := w.ph.setNumArg(part.numArg()); err != nil {
			return err
		}
		w.ph.bufferLength = int32(size)
		w.ph.bufferSize = int32(bufferSize)

		if err := w.ph.encode(w.enc); err != nil {
			return err
		}
		w.tracer(true, w.ph)

		if err := part.encode(w.enc); err != nil {
			return err
		}
		w.tracer(true, part)

		w.enc.Zeroes(pad)

		bufferSize -= int64(partHeaderSize + size + pad)
	}
	return w.wr.Flush()
}
