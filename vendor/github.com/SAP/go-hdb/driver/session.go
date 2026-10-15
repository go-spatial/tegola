package driver

import (
	"bufio"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
	"github.com/SAP/go-hdb/driver/internal/trace"
	"golang.org/x/text/transform"
)

// SessionUser provides the fields for a hdb 'connect' (switch user) statement.
type SessionUser struct {
	Username, Password string
	Schema             string
}

func (u *SessionUser) equal(cmp *SessionUser) bool {
	if cmp == nil {
		return false
	}
	return *u == *cmp
}

func (u *SessionUser) clone() *SessionUser {
	return &SessionUser{Username: u.Username, Password: u.Password, Schema: u.Schema}
}

// use unexported type to avoid key collisions.
type switchUserCtxKeyType struct{}

var switchUserCtxKey switchUserCtxKeyType

// WithUserSwitch can be used to switch a user on a new or an existing connection
// (see https://help.sap.com/docs/hana-cloud-database/sap-hana-cloud-sap-hana-database-sql-reference-guide/connect-statement-session-management).
func WithUserSwitch(ctx context.Context, u *SessionUser) context.Context {
	if u == nil {
		panic("cannot create context from nil SessionUser")
	}
	return context.WithValue(ctx, switchUserCtxKey, u)
}

type session struct {
	metrics        *metrics
	routing        *routing
	routingVersion int64
	attrs          *connAttrs

	readerAttrs *p.ReaderAttrs
	writerAttrs *p.WriterAttrs

	// reusable decoder for reading lob values (cesu8 -> utf8). Reads on a
	// connection are strictly sequential (database/sql serializes per connection
	// and each lob read completes within one Scan call), so there is never
	// concurrent access to reset/clobber and a single instance can be reused
	// instead of minting one per lob read. Only the encode-side lob path, where
	// readers can interleave, needs its own instance per use (see
	// p.LobInDescr.FetchFirst).
	lobReadDecoder transform.Transformer

	prd *p.Reader
	pwr *p.Writer

	hdbVersion   *Version
	databaseName string
	host         string // HANA server dialed host and port.
	serverConnID int    // HANA server connection id (connect option coConnectionID).

	user *SessionUser // session user

	// atomic as data race got reported on closeTx + exec in parallel
	inTx atomic.Bool

	sqlTracer *sqlTracer

	/*
		bad connection flag (can be set by 'done' and 'write' concurrently).
		we cannot work with nested errors containing driver.ErrBadConn
		as go sql retries these statements.
	*/
	canceled bool
}

func newSession(ctx context.Context, conn io.ReadWriter, logger *slog.Logger, metrics *metrics, routing *routing, attrs *connAttrs) (*session, error) {
	protTrace := protTrace.Load()

	readerAttrs := p.NewReaderAttrs(protTrace, logger, attrs.cesu8DecoderFn, attrs.lobChunkSize, attrs.emptyDateAsNull, attrs.compressor)
	writerAttrs := p.NewWriterAttrs(protTrace, logger, attrs.cesu8EncoderFn, attrs.sessionVariables, attrs.compressor)

	// buffer reader
	prd := p.NewDBReader(bufio.NewReaderSize(conn, attrs.bufferSize), readerAttrs)
	pwr := p.NewWriter(conn, writerAttrs)

	// prolog
	if err := pwr.WriteProlog(ctx); err != nil {
		return nil, err
	}
	if err := prd.ReadProlog(ctx); err != nil {
		return nil, err
	}

	var sqlTracer *sqlTracer
	if sqlTrace.Load() {
		sqlTracer = newSQLTracer(logger, 0)
	}

	return &session{
		metrics:        metrics,
		routing:        routing,
		routingVersion: -1,
		attrs:          attrs,
		readerAttrs:    readerAttrs,
		writerAttrs:    writerAttrs,
		lobReadDecoder: attrs.cesu8DecoderFn(),
		prd:            prd,
		pwr:            pwr,
		sqlTracer:      sqlTracer,
	}, nil
}

func (s *session) authenticate(ctx context.Context, host string, authHnd *p.AuthHnd) error {
	cco := &p.ConnectOptions{} // client connect options
	cco.SetDataFormatVersion2(s.attrs.dfv)
	if s.attrs.connectionRouting {
		cco.SetClientDistributionMode(p.CdmConnection)
		cco.SetUpdateTopologyAnywhere(true)
	} else {
		cco.SetClientDistributionMode(p.CdmOff)
	}

	if s.attrs.locale != "" {
		cco.SetClientLocale(s.attrs.locale)
	}

	// Compression negotiation.
	//
	// This driver mirrors the SAP HANA C++ client (hdbcli) and node-hdb
	// reference implementations.
	//
	// The connect option coCompressionLevelAndFlags carries an int32
	// bitfield with two relevant flags:
	//
	//   coCompressionLZ4Supported  (0x00000100) — sender can decompress LZ4
	//   coCompressionLZ4Enabled    (0x00000200) — sender wants compression on
	//
	// Receive side. No per-session decision is needed: every inbound packet
	// carries an isCompressed bit in the message header, which we honour
	// individually.
	//
	// Connect request. Mirrors the C++ and node-hdb clients:
	//   - CompressDisabled     → option omitted entirely; server cannot
	//                            send compressed packets because we did
	//                            not advertise LZ4Supported.
	//   - CompressEnabled      → send LZ4Supported | LZ4Enabled.
	//   - CompressDefault      → send LZ4Supported only; let the server
	//                            decide.
	//
	// Send side. We compress our outbound packets iff the server's connect
	// reply has coCompressionLZ4Supported
	//
	// Bits 0..7 (level), bit 10 (ForceLocal) are unused by this driver.
	// ForceLocal exists in the C++ client as an internal undocumented
	// option (SQLDBC_INTERNAL_CONNECTPROPERTY_COMPRESSLOCAL) to force
	// compression on loopback connections; node-hdb does not implement
	// it. We do not support it for now; can be added later if a concrete
	// need arises.
	if s.attrs.compressor != nil {
		if s.attrs.compressor.EnableWrite() {
			cco.SetCompressionLevelAndFlags(p.CoCompressionLZ4Supported | p.CoCompressionLZ4Enabled)
		} else {
			cco.SetCompressionLevelAndFlags(p.CoCompressionLZ4Supported)
		}
	}
	// else disable compression: option omitted entirely

	sco, ti, err := s._authenticate(ctx, authHnd, cco)
	if err != nil {
		return err
	}

	enabled := sco.ClientDistributionModeOrZero() != p.CdmOff
	s.routingVersion = s.routing.updateFromConnect(enabled, host, sco.DatabaseNameOrZero(), sco.SystemIDOrZero(), ti)

	s.hdbVersion = parseVersion(sco.FullVersionOrZero())
	s.databaseName = sco.DatabaseNameOrZero()
	s.host = host
	s.serverConnID = sco.ServerConnectionIDOrZero()

	s.readerAttrs.SetAlphanumDfv1(sco.DataFormatVersion2OrZero() == p.DfvLevel1)

	compressLevelAndFlags := sco.CompressionLevelAndFlagsOrZero()
	if compressLevelAndFlags&p.CoCompressionLZ4Supported != 0 {
		s.writerAttrs.SetCompressEnableWrite(s.attrs.compressor != nil && s.attrs.compressor.EnableWrite())
	}

	return s.setSchema(ctx)
}

// we cannot work with nested errors containing driver.ErrBadConn
// as go sql retries these statements.
func (s *session) isBad() bool { return s.canceled || s.pwr.HasError() }
func (s *session) cancel()     { s.canceled = true }

func (s *session) close() error {
	// do not disconnect if isBad.
	if !s.isBad() {
		return s.disconnect(context.Background())
	}
	return nil
}

func (s *session) _authenticate(ctx context.Context, authHnd *p.AuthHnd, co *p.ConnectOptions) (*p.ConnectOptions, *p.TopologyInformation, error) {
	defer s.metrics.addTimeValue(timeAuth, time.Now())

	// client context
	clientContext := &p.ClientContext{}
	clientContext.SetVersion(DriverVersion)
	clientContext.SetType(clientType)
	clientContext.SetApplicationProgram(s.attrs.applicationName)

	initRequest, err := authHnd.InitRequest()
	if err != nil {
		return nil, nil, err
	}
	if err := s.pwr.Write(ctx, p.MtAuthenticate, false, clientContext, initRequest); err != nil {
		return nil, nil, err
	}

	initReply, err := authHnd.InitReply()
	if err != nil {
		return nil, nil, err
	}

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			err = pi.ReadHDBErrors(ctx)
		case p.PkAuthentication:
			err = pi.ReadPart(ctx, initReply)
		default:
			err = pi.SkipPart(ctx)
		}
		if err != nil {
			return nil, nil, err
		}
	}

	finalRequest, err := authHnd.FinalRequest()
	if err != nil {
		return nil, nil, err
	}

	if err := s.pwr.Write(ctx, p.MtConnect, false, finalRequest, p.ClientID(clientID), co); err != nil {
		return nil, nil, err
	}

	finalReply, err := authHnd.FinalReply()
	if err != nil {
		return nil, nil, err
	}

	ti := new(p.TopologyInformation)

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			err = pi.ReadHDBErrors(ctx)
		case p.PkAuthentication:
			err = pi.ReadPart(ctx, finalReply)
		case p.PkConnectOptions:
			err = pi.ReadPart(ctx, co)
		case p.PkTopologyInformation:
			err = pi.ReadPart(ctx, ti)
		default:
			err = pi.SkipPart(ctx)
		}
		if err != nil {
			return nil, nil, err
		}
	}

	sessionID := s.prd.SessionID()
	if sessionID <= 0 {
		return nil, nil, fmt.Errorf("invalid session id %d", sessionID)
	}
	s.pwr.SetSessionID(sessionID)
	return co, ti, nil
}

func (s *session) setSchema(ctx context.Context) error {
	switch {
	case s.user != nil && s.user.Schema != "":
		_, err := s.execDirect(ctx, "set schema "+Identifier(s.user.Schema).String())
		return err
	case s.attrs.defaultSchema != "":
		_, err := s.execDirect(ctx, "set schema "+Identifier(s.attrs.defaultSchema).String())
		return err
	default:
		return nil
	}
}

// ErrSwitchUser is the error raised if a switch user is requested in a disallowed context.
var ErrSwitchUser = errors.New("switch user inside transaction or in statement scope (prepared query) is not allowed")

func (s *session) switchUser(ctx context.Context) error {
	user, ok := ctx.Value(switchUserCtxKey).(*SessionUser)
	if !ok || user.equal(s.user) {
		return nil
	}
	if s.inTx.Load() {
		return ErrSwitchUser
	}
	connectQuery := func(password string) string {
		return "connect " + user.Username + " password \"" + password + "\""
	}
	if _, err := s.execDirectQueryLog(ctx, connectQuery(user.Password), connectQuery(trace.RedactedText)); err != nil {
		return err
	}
	s.user = user.clone()
	s.metrics.addCounter(counterSessionConnects, 1)
	return s.setSchema(ctx)
}

func (s *session) preventSwitchUser(ctx context.Context) error {
	user, ok := ctx.Value(switchUserCtxKey).(*SessionUser)
	if !ok || user.equal(s.user) {
		return nil
	}
	return ErrSwitchUser
}

func (s *session) dbConnectInfo(ctx context.Context, databaseName string) (*DBConnectInfo, error) {
	ci := &p.DBConnectInfo{}
	ci.SetDatabaseName(databaseName)
	if err := s.pwr.Write(ctx, p.MtDBConnectInfo, false, ci); err != nil {
		return nil, err
	}

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			err = pi.ReadHDBErrors(ctx)
		case p.PkDBConnectInfo:
			err = pi.ReadPart(ctx, ci)
		default:
			err = pi.SkipPart(ctx)
		}
		if err != nil {
			return nil, err
		}
	}

	return &DBConnectInfo{
		DatabaseName: databaseName,
		Host:         ci.HostOrZero(),
		Port:         ci.PortOrZero(),
		IsConnected:  ci.IsConnectedOrZero(),
	}, nil
}

func (s *session) updateRouting(ctx context.Context, pi *p.PartInfo) {
	if s.routingVersion == -1 {
		return
	}
	ti := new(p.TopologyInformation)
	if err := pi.ReadPart(ctx, ti); err != nil {
		return // ignore error
	}
	s.routing.updateFromReply(s.routingVersion, ti)
}

func (s *session) queryDirect(ctx context.Context, query string, traceKind string) (driver.Rows, error) {
	t := time.Now()
	defer s.metrics.addSQLTimeValue(sqlTimeQuery, time.Now())

	// allow e.g inserts as query -> handle commit like in _execDirect
	if err := s.pwr.Write(ctx, p.MtExecuteDirect, !s.inTx.Load(), p.Command(query)); err != nil {
		return nil, err
	}

	qrs := []*queryResult{}
	var qr *queryResult
	meta := &p.ResultMetadata{}
	resSet := &p.Resultset{}

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			if err := pi.ReadHDBErrors(ctx); err != nil {
				return nil, err
			}
		case p.PkResultMetadata:
			qr = &queryResult{session: s}
			qrs = append(qrs, qr)
			if err := pi.ReadPart(ctx, meta); err != nil {
				return nil, err
			}
			qr.fields = meta.ResultFields
		case p.PkResultsetID:
			if err := pi.ReadPart(ctx, (*p.ResultsetID)(&qr.rsID)); err != nil {
				return nil, err
			}
		case p.PkResultset:
			resSet.ResultFields = qr.fields
			if err := readResultPart(ctx, pi, resSet, qr); err != nil {
				return nil, err
			}
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attrs = pi.Header.Attrs()
		case p.PkTopologyInformation:
			s.updateRouting(ctx, pi)
		default:
			if err := pi.SkipPart(ctx); err != nil {
				return nil, err
			}
		}
	}
	if s.sqlTracer != nil {
		s.sqlTracer.log(ctx, t, traceKind, query)
	}
	if !slices.ContainsFunc(qrs, func(qr *queryResult) bool { // no select query
		return qr.rsID != 0
	}) {
		return noResult, nil
	}
	if len(qrs) > 1 {
		return &queryMultiResult{qrs: qrs}, nil
	}
	return qr, nil
}

func (s *session) execDirectQueryLog(ctx context.Context, query, logQuery string) (driver.Result, error) {
	t := time.Now()
	defer s.metrics.addSQLTimeValue(sqlTimeExec, time.Now())

	if err := s.pwr.Write(ctx, p.MtExecuteDirect, !s.inTx.Load(), p.Command(query)); err != nil {
		return nil, err
	}

	rowsAffected := new(p.RowsAffected)

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			err = pi.ReadHDBErrors(ctx)
		case p.PkRowsAffected:
			err = pi.ReadPart(ctx, rowsAffected)
		case p.PkTopologyInformation:
			s.updateRouting(ctx, pi)
		default:
			err = pi.SkipPart(ctx)
		}
		if err != nil {
			return nil, err
		}
	}
	numRow := rowsAffected.Total()

	if s.sqlTracer != nil {
		s.sqlTracer.log(ctx, t, traceExec, logQuery)
	}
	if s.prd.FunctionCode() == p.FcDDL {
		return driver.ResultNoRows, nil
	}
	return driver.RowsAffected(numRow), nil
}

func (s *session) execDirect(ctx context.Context, query string) (driver.Result, error) {
	return s.execDirectQueryLog(ctx, query, query)
}

func (s *session) prepare(ctx context.Context, query string) (*prepareResult, error) {
	t := time.Now()
	defer s.metrics.addSQLTimeValue(sqlTimePrepare, time.Now())

	if err := s.pwr.Write(ctx, p.MtPrepare, false, p.Command(query)); err != nil {
		return nil, err
	}

	pr := &prepareResult{}
	resMeta := &p.ResultMetadata{}
	prmMeta := &p.ParameterMetadata{}

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			if err := pi.ReadHDBErrors(ctx); err != nil {
				return nil, err
			}
		case p.PkStatementID:
			if err := pi.ReadPart(ctx, (*p.StatementID)(&pr.stmtID)); err != nil {
				return nil, err
			}
		case p.PkResultMetadata:
			if err := pi.ReadPart(ctx, resMeta); err != nil {
				return nil, err
			}
			pr.resultFields = resMeta.ResultFields
		case p.PkParameterMetadata:
			if err := pi.ReadPart(ctx, prmMeta); err != nil {
				return nil, err
			}
			pr.parameterFields = prmMeta.ParameterFields
		default:
			if err := pi.SkipPart(ctx); err != nil {
				return nil, err
			}
		}
	}
	pr.fc = s.prd.FunctionCode()
	if s.sqlTracer != nil {
		s.sqlTracer.log(ctx, t, tracePrepare, query)
	}
	return pr, nil
}

func (s *session) query(ctx context.Context, query string, pr *prepareResult, nvargs []driver.NamedValue) (driver.Rows, error) {
	t := time.Now()
	defer s.metrics.addSQLTimeValue(sqlTimeQuery, time.Now())

	// allow e.g inserts as query -> handle commit like in exec

	if err := convertQueryArgs(pr.parameterFields, nvargs, s.attrs.cesu8EncoderFn, s.attrs.lobChunkSize); err != nil {
		return nil, err
	}
	inputParameters := p.NewInputParameters(pr.parameterFields, nvargs)
	if err := s.pwr.Write(ctx, p.MtExecute, !s.inTx.Load(), p.StatementID(pr.stmtID), inputParameters); err != nil {
		return nil, err
	}

	qr := &queryResult{session: s, fields: pr.resultFields}
	resSet := &p.Resultset{}

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			if err := pi.ReadHDBErrors(ctx); err != nil {
				return nil, err
			}
		case p.PkResultsetID:
			if err := pi.ReadPart(ctx, (*p.ResultsetID)(&qr.rsID)); err != nil {
				return nil, err
			}
		case p.PkResultset:
			resSet.ResultFields = qr.fields
			if err := readResultPart(ctx, pi, resSet, qr); err != nil {
				return nil, err
			}
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attrs = pi.Header.Attrs()
		case p.PkTopologyInformation:
			s.updateRouting(ctx, pi)
		default:
			if err := pi.SkipPart(ctx); err != nil {
				return nil, err
			}
		}
	}
	if s.sqlTracer != nil {
		s.sqlTracer.log(ctx, t, traceQuery, query, nvargs...)
	}
	if qr.rsID == 0 { // non select query
		return noResult, nil
	}
	return qr, nil
}

func (s *session) exec(ctx context.Context, query string, pr *prepareResult, nvargs []driver.NamedValue, offset int) (driver.Result, error) {
	t := time.Now()
	defer s.metrics.addSQLTimeValue(sqlTimeExec, time.Now())

	inputParameters := p.NewInputParameters(pr.parameterFields, nvargs)
	if err := s.pwr.Write(ctx, p.MtExecute, !s.inTx.Load(), p.StatementID(pr.stmtID), inputParameters); err != nil {
		return nil, err
	}

	var ids []p.LocatorID
	lobReply := &p.WriteLobReply{}
	rowsAffected := new(p.RowsAffected)

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			if err := pi.ReadHDBErrors(ctx); err != nil {
				if hdbErrors, ok := errors.AsType[*p.HdbErrors](err); ok {
					rowsAffected.SetHDbErrorsStmtNo(hdbErrors, offset)
				}
				return nil, err
			}
		case p.PkWriteLobReply:
			if err := pi.ReadPart(ctx, lobReply); err != nil {
				return nil, err
			}
			ids = lobReply.IDs
		case p.PkRowsAffected:
			if err := pi.ReadPart(ctx, rowsAffected); err != nil {
				return nil, err
			}
		case p.PkTopologyInformation:
			s.updateRouting(ctx, pi)
		default:
			if err := pi.SkipPart(ctx); err != nil {
				return nil, err
			}
		}
	}
	fc := s.prd.FunctionCode()
	numRow := rowsAffected.Total()

	if len(ids) != 0 {
		/*
			writeLobParameters:
			- chunkReaders
			- nil (no callResult, exec does not have output parameters)
		*/

		/*
			write lob data only for the last record as lob streaming is only available for the last one
		*/
		startLastRec := len(nvargs) - len(pr.parameterFields)
		numlobRow, err := s.writeLobs(ctx, ids, pr.parameterFields, nvargs[startLastRec:])
		if err != nil {
			return nil, err
		}
		// Accumulate LOB rows with the initial numRow.
		// HANA 2: lobRowsAffected will be 0
		// HANA 4: lobRowsAffected will be > 0
		numRow += numlobRow
	}
	if s.sqlTracer != nil {
		s.sqlTracer.log(ctx, t, traceExec, query, nvargs...)
	}
	if fc == p.FcDDL {
		return driver.ResultNoRows, nil
	}
	return driver.RowsAffected(numRow), nil
}

func (s *session) execCall(ctx context.Context, query string, pr *prepareResult, nvargs []driver.NamedValue) (*callResult, *callArgs, int64, error) {
	t := time.Now()
	defer s.metrics.addSQLTimeValue(sqlTimeCall, time.Now())

	callArgs, err := convertCallArgs(pr.parameterFields, nvargs, s.attrs.cesu8EncoderFn, s.attrs.lobChunkSize)
	if err != nil {
		return nil, nil, 0, err
	}
	inputParameters := p.NewInputParameters(callArgs.inFields, callArgs.inArgs)
	if err := s.pwr.Write(ctx, p.MtExecute, !s.inTx.Load(), (*p.StatementID)(&pr.stmtID), inputParameters); err != nil {
		return nil, nil, 0, err
	}

	cr := &callResult{session: s, outFields: callArgs.outFields}

	var qr *queryResult
	var ids []p.LocatorID
	outPrms := &p.OutputParameters{}
	meta := &p.ResultMetadata{}
	resSet := &p.Resultset{}
	lobReply := &p.WriteLobReply{}
	rowsAffected := new(p.RowsAffected)
	tableRowIdx := 0

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return nil, nil, 0, err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			if err := pi.ReadHDBErrors(ctx); err != nil {
				return nil, nil, 0, err
			}
		case p.PkOutputParameters:
			outPrms.OutputFields = cr.outFields
			if err := readResultPart(ctx, pi, outPrms, cr); err != nil {
				return nil, nil, 0, err
			}
			cr.fieldValues = outPrms.FieldValues
			cr.decodeErrors = outPrms.DecodeErrors
		case p.PkResultMetadata:
			/*
				procedure call with table parameters does return metadata for each table
				sequence: metadata, resultsetID, resultset
				but:
				- resultset might not be provided for all tables
				- so, 'additional' query result is detected by new metadata part
			*/
			qr = &queryResult{session: s}
			cr.outFields = append(cr.outFields, p.NewTableRowsParameterField(tableRowIdx))
			cr.fieldValues = append(cr.fieldValues, qr)
			tableRowIdx++
			if err := pi.ReadPart(ctx, meta); err != nil {
				return nil, nil, 0, err
			}
			qr.fields = meta.ResultFields
		case p.PkResultset:
			resSet.ResultFields = qr.fields
			if err := readResultPart(ctx, pi, resSet, qr); err != nil {
				return nil, nil, 0, err
			}
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attrs = pi.Header.Attrs()
		case p.PkResultsetID:
			if err := pi.ReadPart(ctx, (*p.ResultsetID)(&qr.rsID)); err != nil {
				return nil, nil, 0, err
			}
		case p.PkWriteLobReply:
			if err := pi.ReadPart(ctx, lobReply); err != nil {
				return nil, nil, 0, err
			}
			ids = lobReply.IDs
		case p.PkRowsAffected:
			if err := pi.ReadPart(ctx, rowsAffected); err != nil {
				return nil, nil, 0, err
			}
		case p.PkTopologyInformation:
			s.updateRouting(ctx, pi)
		default:
			if err := pi.SkipPart(ctx); err != nil {
				return nil, nil, 0, err
			}
		}
	}
	numRow := rowsAffected.Total()

	if len(ids) != 0 {
		/*
			writeLobParameters:
			- chunkReaders
			- cr (callResult output parameters are set after all lob input parameters are written)
		*/
		numLobRow, err := s.writeLobs(ctx, ids, callArgs.inFields, callArgs.inArgs)
		if err != nil {
			return nil, nil, 0, err
		}
		// Accumulate LOB rows with the initial numRow.
		// HANA 2: lobRowsAffected will be 0
		// HANA 4: lobRowsAffected will be > 0
		numRow += numLobRow
	}
	if s.sqlTracer != nil {
		s.sqlTracer.log(ctx, t, traceExecCall, query, nvargs...)
	}
	return cr, callArgs, numRow, nil
}

func (s *session) fetchNext(ctx context.Context, qr *queryResult) error {
	defer s.metrics.addSQLTimeValue(sqlTimeFetch, time.Now())

	if err := s.pwr.Write(ctx, p.MtFetchNext, false, p.ResultsetID(qr.rsID), p.Fetchsize(s.attrs.fetchSize)); err != nil { //nolint: gosec
		return err
	}

	resSet := &p.Resultset{ResultFields: qr.fields, FieldValues: qr.fieldValues} // reuse field values

	for pi, err := range s.prd.Parts(ctx) {
		if err != nil {
			return err
		}
		switch pi.Header.Kind() {
		case p.PkError:
			if err := pi.ReadHDBErrors(ctx); err != nil {
				return err
			}
		case p.PkResultset:
			if err := readResultPart(ctx, pi, resSet, qr); err != nil {
				return err
			}
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attrs = pi.Header.Attrs()
		case p.PkTopologyInformation:
			s.updateRouting(ctx, pi)
		default:
			if err := pi.SkipPart(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *session) dropStatementID(ctx context.Context, id uint64) error {
	if err := s.pwr.Write(ctx, p.MtDropStatementID, false, p.StatementID(id)); err != nil {
		return err
	}
	return s.prd.SkipParts(ctx)
}

func (s *session) closeResultsetID(ctx context.Context, id uint64) error {
	if err := s.pwr.Write(ctx, p.MtCloseResultset, false, p.ResultsetID(id)); err != nil {
		return err
	}
	return s.prd.SkipParts(ctx)
}

func (s *session) commit(ctx context.Context) error {
	defer s.metrics.addSQLTimeValue(sqlTimeCommit, time.Now())

	if err := s.pwr.Write(ctx, p.MtCommit, false); err != nil {
		return err
	}
	if err := s.prd.SkipParts(ctx); err != nil {
		return err
	}
	return nil
}

func (s *session) rollback(ctx context.Context) error {
	defer s.metrics.addSQLTimeValue(sqlTimeRollback, time.Now())

	if err := s.pwr.Write(ctx, p.MtRollback, false); err != nil {
		return err
	}
	if err := s.prd.SkipParts(ctx); err != nil {
		return err
	}
	return nil
}

func (s *session) disconnect(ctx context.Context) error {
	/*
		Do not send a disconnect message. The reference hdbcli implementation closes
		the TCP connection without a disconnect message and HANA terminates the session
		on socket close. As conn.Close closes the connection right after this call, the
		message would race the close and the server reply is unreliably delivered, so a
		disconnect message adds no value.

		// if err := s.pwr.Write(ctx, p.MtDisconnect, false); err != nil {
		// 	return err
		// }

		Do not read server reply as on slow connections the TCP/IP connection is closed (by Server)
		before the reply can be read completely.

		// if err := s.pr.readSkip(); err != nil {
		// 	return err
		// }

	*/
	return nil
}

// writeLobs writes input lob parameters to db and returns the accumulated rows.
func (s *session) writeLobs(ctx context.Context, ids []p.LocatorID, inPrmFields []*p.ParameterField, nvargs []driver.NamedValue) (int64, error) {
	if len(inPrmFields) != len(nvargs) {
		panic("lob streaming can only be done for one (the last) record")
	}
	descrs := make([]*p.WriteLobDescr, 0, len(ids))
	j := 0
	for i, f := range inPrmFields {
		if f.IsLob() && nvargs[i].Value != nil {
			lobInDescr, ok := nvargs[i].Value.(*p.LobInDescr)
			if !ok {
				return 0, fmt.Errorf("protocol error: invalid lob parameter %[1]T %[1]v - lobInDescr expected", nvargs[i])
			}
			if !lobInDescr.IsLastData() {
				if j >= len(ids) {
					return 0, fmt.Errorf("protocol error: id index %d out of range - number of ids %d", j, len(ids))
				}
				descrs = append(descrs, &p.WriteLobDescr{LobInDescr: lobInDescr, ID: ids[j]})
				j++
			}
		}
	}

	var totalNumRow int64
	writeLobRequest := &p.WriteLobRequest{}
	for len(descrs) != 0 {

		if len(descrs) != len(ids) {
			return 0, fmt.Errorf("protocol error: invalid number of lob parameter ids %d - expected %d", len(descrs), len(ids))
		}
		for i, descr := range descrs { // check if ids and descrs are in sync
			if descr.ID != ids[i] {
				return 0, fmt.Errorf("protocol error: lob parameter id mismatch %d - expected %d", descr.ID, ids[i])
			}
		}

		// TODO check total size limit
		for _, descr := range descrs {
			if err := descr.FetchNext(s.attrs.lobChunkSize); err != nil {
				return 0, err
			}
		}

		writeLobRequest.Descrs = descrs

		if err := s.pwr.Write(ctx, p.MtReadLob, false, writeLobRequest); err != nil {
			return 0, err
		}

		lobReply := &p.WriteLobReply{}
		rowsAffected := new(p.RowsAffected)

		for pi, err := range s.prd.Parts(ctx) {
			if err != nil {
				return 0, err
			}
			switch pi.Header.Kind() {
			case p.PkError:
				if err := pi.ReadHDBErrors(ctx); err != nil {
					return 0, err
				}
			case p.PkWriteLobReply:
				if err := pi.ReadPart(ctx, lobReply); err != nil {
					return 0, err
				}
				ids = lobReply.IDs
			case p.PkRowsAffected:
				if err := pi.ReadPart(ctx, rowsAffected); err != nil {
					return 0, err
				}
			default:
				if err := pi.SkipPart(ctx); err != nil {
					return 0, err
				}
			}
		}

		// Accumulate rowsAffected from LOB write operations.
		// HANA 2: returns 0 for LOB writes
		// HANA 4: returns 1 for LOB writes
		totalNumRow += rowsAffected.Total()

		// remove done descr
		j := 0
		for _, descr := range descrs {
			if !descr.IsLastData() {
				descrs[j] = descr
				j++
			}
		}
		descrs = descrs[:j]
	}
	return totalNumRow, nil
}
