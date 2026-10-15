package driver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
	"github.com/SAP/go-hdb/driver/internal/protocol/auth"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
)

// A Sniffer is a simple proxy for logging hdb protocol requests and responses.
type Sniffer struct {
	logger *slog.Logger
	conn   net.Conn
	dbConn net.Conn
}

// NewSniffer creates a new sniffer instance. The conn parameter is the net.Conn connection, where the Sniffer
// is listening for hdb protocol calls. The dbConn is the hdb connection to the database.
func NewSniffer(conn net.Conn, dbConn net.Conn) *Sniffer {
	return &Sniffer{
		logger: slog.Default().With(slog.String("conn", conn.RemoteAddr().String())),
		conn:   conn,
		dbConn: dbConn,
	}
}

type snifferReaderState struct {
	resultFields    []*p.ResultField
	parameterFields []*p.ParameterField
	connectOptions  *p.ConnectOptions
	isClient        bool
	authCount       int
	authMethod      auth.Method // selected auth method
}

func filterFields(fields []*p.ParameterField, out bool) []*p.ParameterField {
	rv := make([]*p.ParameterField, 0, len(fields))
	for _, f := range fields {
		if (out && f.Out()) || (!out && f.In()) {
			rv = append(rv, f)
		}
	}
	return rv
}

func readMsg(ctx context.Context, prd *p.Reader, state *snifferReaderState, params map[p.StatementID][]*p.ParameterField, lob *lobLocator) error {
	var stmtID p.StatementID // statement id of the current message, if present
	for pi, err := range prd.Parts(ctx) {
		if err != nil {
			return err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			err = pi.ReadHDBErrors(ctx)
			if err != nil {
				// HdbErrors result must not abort, a real decode error must.
				if _, ok := errors.AsType[*p.HdbErrors](err); ok {
					err = nil
				}
			}
		case p.PkStatementID:
			err = pi.ReadPart(ctx, &stmtID)
		case p.PkResultMetadata:
			meta := new(p.ResultMetadata)
			if err = pi.ReadPart(ctx, meta); err == nil {
				state.resultFields = meta.ResultFields
			}
		case p.PkResultset:
			err = snifferReadResultPart(ctx, pi, &p.Resultset{ResultFields: state.resultFields})
		case p.PkParameterMetadata:
			meta := new(p.ParameterMetadata)
			if err = pi.ReadPart(ctx, meta); err == nil {
				state.parameterFields = meta.ParameterFields
				params[stmtID] = meta.ParameterFields
			}
		case p.PkConnectOptions:
			co := new(p.ConnectOptions)
			if err = pi.ReadPart(ctx, co); err == nil {
				state.connectOptions = co // negotiate compression and data format version
			}
		case p.PkOutputParameters:
			err = snifferReadResultPart(ctx, pi, &p.OutputParameters{OutputFields: filterFields(state.parameterFields, true)})
		case p.PkParameters:
			if fields, ok := params[stmtID]; ok && len(fields) != 0 {
				err = pi.ReadPart(ctx, &p.InputParameters{InputFields: filterFields(fields, false)})
			} else {
				err = pi.SkipPart(ctx) // no metadata -> cannot decode
			}
		case p.PkReadLobRequest:
			req := new(p.ReadLobRequest)
			if err = pi.ReadPart(ctx, req); err == nil {
				lob.id, lob.set = req.ID, true
			}
		case p.PkReadLobReply:
			if lob.set {
				err = pi.ReadPart(ctx, p.NewReadLobReply(lob.id))
			} else {
				err = pi.SkipPart(ctx) // no request -> cannot decode
			}
		case p.PkAuthentication:
			state.authCount++
			switch {
			case state.isClient && state.authCount == 1:
				err = pi.ReadPart(ctx, new(p.AuthInitRequest))
			case state.isClient && state.authCount == 2:
				err = pi.ReadPart(ctx, new(p.AuthFinalRequest))
			case !state.isClient && state.authCount == 1:
				rep := new(p.AuthInitReply)
				if err = pi.ReadPart(ctx, rep); err == nil {
					state.authMethod = rep.Method // selected method, decodes the final reply
				}
			case !state.isClient && state.authCount == 2 && state.authMethod != nil:
				err = pi.ReadPart(ctx, &p.AuthFinalReply{Method: state.authMethod})
			default:
				err = pi.SkipPart(ctx)
			}
			// Verification errors (ErrAuthVerifyFailed) must not abort, a real decode error must.
			if err != nil && errors.Is(err, auth.ErrAuthVerifyFailed) {
				err = nil
			}
		default:
			err = pi.SkipPart(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// bridge forwards the remaining messages of both directions without tracing.
// It is used once compression is negotiated as compressed packets cannot be traced.
func (s *Sniffer) bridge(clientRd, dbRd *p.Reader) error {
	s.logger.Info("connection compression negotiated - bridged mode, tracing disabled")
	for {
		if err := clientRd.SkipMessage(); err != nil {
			return err
		}
		if err := dbRd.SkipMessage(); err != nil {
			return err
		}
	}
}

type lobLocator struct {
	id  p.LocatorID
	set bool
}

// Run starts the sniffer.
// Both directions are processed sequentially in a single routine, mirroring
// the server's strict request-reply cycle.
func (s *Sniffer) Run() error {
	ctx := context.Background()
	readerAttrs := p.NewReaderAttrs(true, s.logger, cesu8.DefaultDecoder, defaultLobChunkSize, false, nil)

	c2d := io.TeeReader(s.conn, s.dbConn) // client request -> database
	d2c := io.TeeReader(s.dbConn, s.conn) // database reply -> client

	clientRd := p.NewClientReader(c2d, readerAttrs)
	dbRd := p.NewDBReader(d2c, readerAttrs)

	clientState := &snifferReaderState{isClient: true}
	dbState := &snifferReaderState{}
	params := make(map[p.StatementID][]*p.ParameterField)
	lob := &lobLocator{}

	if err := clientRd.ReadProlog(ctx); err != nil {
		return err
	}
	if err := dbRd.ReadProlog(ctx); err != nil {
		return err
	}

	compression := false
	for {
		if err := readMsg(ctx, clientRd, clientState, params, lob); err != nil {
			return err
		}
		if co := clientState.connectOptions; co != nil {
			compression = compression || co.CompressionLevelAndFlagsOrZero()&p.CoCompressionLZ4Supported != 0
			clientState.connectOptions = nil
		}
		if err := readMsg(ctx, dbRd, dbState, params, lob); err != nil {
			return err
		}
		if co := dbState.connectOptions; co != nil {
			if co.DataFormatVersion2OrZero() == p.DfvLevel1 {
				readerAttrs.SetAlphanumDfv1(true)
			}
			compression = compression || co.CompressionLevelAndFlagsOrZero()&p.CoCompressionLZ4Supported != 0
			dbState.connectOptions = nil
		}
		if compression {
			return s.bridge(clientRd, dbRd)
		}
	}
}
