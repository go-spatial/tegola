package driver

// TODO Sniffer
/*
sniffer:
- complete for go-hdb: especially call with table parameters
- delete caches for statement and result
- don't ignore part read error
  - example: read scramsha256InitialReply got silently stuck because methodname check failed
- test with python client and handle surprises
  - analyze for not ignoring part read errors
*/

import (
	"bufio"
	"io"
	"net"
	"sync"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
)

// A Sniffer is a simple proxy for logging hdb protocol requests and responses.
type Sniffer struct {
	conn   net.Conn
	dbConn net.Conn

	//client
	clRd *bufio.Reader
	clWr *bufio.Writer
	//database
	dbRd *bufio.Reader
	dbWr *bufio.Writer

	// reader
	upRd   *sniffUpReader
	downRd *sniffDownReader
}

// NewSniffer creates a new sniffer instance. The conn parameter is the net.Conn connection, where the Sniffer
// is listening for hdb protocol calls. The dbAddr is the hdb host port address in "host:port" format.
func NewSniffer(conn net.Conn, dbConn net.Conn) *Sniffer {

	//TODO - review setting values here
	//protocolTraceFlag.Set("true")

	s := &Sniffer{
		conn:   conn,
		dbConn: dbConn,
		// buffered write to client
		clWr: bufio.NewWriter(conn),
		// buffered write to db
		dbWr: bufio.NewWriter(dbConn),
	}

	//read from client connection and write to db buffer
	s.clRd = bufio.NewReader(io.TeeReader(conn, s.dbWr))
	//read from db and write to client connection buffer
	s.dbRd = bufio.NewReader(io.TeeReader(dbConn, s.clWr))

	s.upRd = newSniffUpReader(s.clRd)
	s.downRd = newSniffDownReader(s.dbRd)

	return s
}

// Run starts the protocol request and response logging.
func (s *Sniffer) Run() error {
	defer s.dbConn.Close()
	defer s.conn.Close()

	if err := s.upRd.pr.ReadProlog(); err != nil {
		return err
	}
	if err := s.dbWr.Flush(); err != nil {
		return err
	}
	if err := s.downRd.pr.ReadProlog(); err != nil {
		return err
	}
	if err := s.clWr.Flush(); err != nil {
		return err
	}

	for {
		//up stream
		if err := s.upRd.readMsg(); err != nil {
			return err // err == io.EOF: connection closed by client
		}
		if err := s.dbWr.Flush(); err != nil {
			return err
		}
		//down stream
		if err := s.downRd.readMsg(); err != nil {
			if _, ok := err.(*p.HdbErrors); !ok { //if hdbErrors continue
				return err
			}
		}
		if err := s.clWr.Flush(); err != nil {
			return err
		}
	}
}

type sniffReader struct {
	pr *p.Reader
}

func newSniffReader(upStream bool, rd *bufio.Reader) *sniffReader {
	return &sniffReader{pr: p.NewReader(upStream, rd, cesu8.DefaultDecoder)}
}

type sniffUpReader struct{ *sniffReader }

func newSniffUpReader(rd *bufio.Reader) *sniffUpReader {
	return &sniffUpReader{sniffReader: newSniffReader(true, rd)}
}

type resMetaCache struct {
	mu    sync.RWMutex
	cache map[uint64]*p.ResultMetadata
}

func newResMetaCache() *resMetaCache {
	return &resMetaCache{cache: make(map[uint64]*p.ResultMetadata)}
}

func (c *resMetaCache) put(stmtID uint64, resMeta *p.ResultMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[stmtID] = resMeta
}

type prmMetaCache struct {
	mu    sync.RWMutex
	cache map[uint64]*p.ParameterMetadata
}

func newPrmMetaCache() *prmMetaCache {
	return &prmMetaCache{cache: make(map[uint64]*p.ParameterMetadata)}
}

func (c *prmMetaCache) put(stmtID uint64, prmMeta *p.ParameterMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[stmtID] = prmMeta
}

func (c *prmMetaCache) get(stmtID uint64) *p.ParameterMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[stmtID]
}

var _resMetaCache = newResMetaCache()
var _prmMetaCache = newPrmMetaCache()

func (r *sniffUpReader) readMsg() error {
	var stmtID uint64

	return r.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkStatementID:
			r.pr.Read((*p.StatementID)(&stmtID))
		// case pkResultMetadata:
		// 	r.pr.read(resMeta)
		case p.PkParameters:
			prmMeta := _prmMetaCache.get(stmtID)
			prms := &p.InputParameters{InputFields: prmMeta.ParameterFields} // TODO only input parameters
			r.pr.Read(prms)
		}
	})
}

type sniffDownReader struct {
	*sniffReader
	resMeta *p.ResultMetadata
	prmMeta *p.ParameterMetadata
}

func newSniffDownReader(rd *bufio.Reader) *sniffDownReader {
	return &sniffDownReader{
		sniffReader: newSniffReader(false, rd),
		resMeta:     &p.ResultMetadata{},
		prmMeta:     &p.ParameterMetadata{},
	}
}

func (r *sniffDownReader) readMsg() error {
	var stmtID uint64
	//resMeta := &resultMetadata{}
	//prmMeta := &parameterMetadata{}

	if err := r.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkStatementID:
			r.pr.Read((*p.StatementID)(&stmtID))
		case p.PkResultMetadata:
			r.pr.Read(r.resMeta)
		case p.PkParameterMetadata:
			r.pr.Read(r.prmMeta)
		case p.PkOutputParameters:
			outFields := []*p.ParameterField{}
			for _, f := range r.prmMeta.ParameterFields {
				if f.Out() {
					outFields = append(outFields, f)
				}
			}
			outPrms := &p.OutputParameters{OutputFields: outFields}
			r.pr.Read(outPrms)
		case p.PkResultset:
			resSet := &p.Resultset{ResultFields: r.resMeta.ResultFields}
			r.pr.Read(resSet)
		}
	}); err != nil {
		return err
	}
	_resMetaCache.put(stmtID, r.resMeta)
	_prmMetaCache.put(stmtID, r.prmMeta)
	return nil
}
