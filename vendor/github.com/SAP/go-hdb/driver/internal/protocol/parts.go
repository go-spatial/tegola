package protocol

import (
	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// Part represents a protocol part.
type Part interface {
	String() string // should support Stringer interface
	kind() PartKind
}

// PartDecoder represents a protocol part decoder.
type PartDecoder interface {
	Part
	decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error
}

// PartEncoder represents a protocol part the driver is able to encode.
type PartEncoder interface {
	Part
	numArg() int
	encode(enc *encoding.Encoder) error
}

func (*HdbErrors) kind() PartKind           { return PkError }
func (*AuthInitRequest) kind() PartKind     { return PkAuthentication }
func (*AuthInitReply) kind() PartKind       { return PkAuthentication }
func (*AuthFinalRequest) kind() PartKind    { return PkAuthentication }
func (*AuthFinalReply) kind() PartKind      { return PkAuthentication }
func (ClientID) kind() PartKind             { return PkClientID }
func (clientInfo) kind() PartKind           { return PkClientInfo }
func (*TopologyInformation) kind() PartKind { return PkTopologyInformation }
func (Command) kind() PartKind              { return PkCommand }
func (*RowsAffected) kind() PartKind        { return PkRowsAffected }
func (StatementID) kind() PartKind          { return PkStatementID }
func (*ParameterMetadata) kind() PartKind   { return PkParameterMetadata }
func (*InputParameters) kind() PartKind     { return PkParameters }
func (*OutputParameters) kind() PartKind    { return PkOutputParameters }
func (*ResultMetadata) kind() PartKind      { return PkResultMetadata }
func (ResultsetID) kind() PartKind          { return PkResultsetID }
func (*Resultset) kind() PartKind           { return PkResultset }
func (Fetchsize) kind() PartKind            { return PkFetchSize }
func (*ReadLobRequest) kind() PartKind      { return PkReadLobRequest }
func (*ReadLobReply) kind() PartKind        { return PkReadLobReply }
func (*WriteLobRequest) kind() PartKind     { return PkWriteLobRequest }
func (*WriteLobReply) kind() PartKind       { return PkWriteLobReply }
func (*ClientContext) kind() PartKind       { return PkClientContext }
func (*ConnectOptions) kind() PartKind      { return PkConnectOptions }
func (*DBConnectInfo) kind() PartKind       { return PkDBConnectInfo }
func (*statementContext) kind() PartKind    { return PkStatementContext }
func (*transactionFlags) kind() PartKind    { return PkTransactionFlags }

// numArg methods (result == 1).
func (*AuthInitRequest) numArg() int  { return 1 }
func (*AuthFinalRequest) numArg() int { return 1 }
func (ClientID) numArg() int          { return 1 }
func (Command) numArg() int           { return 1 }
func (StatementID) numArg() int       { return 1 }
func (ResultsetID) numArg() int       { return 1 }
func (Fetchsize) numArg() int         { return 1 }
func (*ReadLobRequest) numArg() int   { return 1 }

// check if part types implement the part encoder interface.
var (
	_ PartEncoder = (*AuthInitRequest)(nil)
	_ PartEncoder = (*AuthFinalRequest)(nil)
	_ PartEncoder = (*ClientID)(nil)
	_ PartEncoder = (*clientInfo)(nil)
	_ PartEncoder = (*Command)(nil)
	_ PartEncoder = (*StatementID)(nil)
	_ PartEncoder = (*InputParameters)(nil)
	_ PartEncoder = (*ResultsetID)(nil)
	_ PartEncoder = (*Fetchsize)(nil)
	_ PartEncoder = (*ReadLobRequest)(nil)
	_ PartEncoder = (*WriteLobRequest)(nil)
	_ PartEncoder = (*ClientContext)(nil)
	_ PartEncoder = (*ConnectOptions)(nil)
	_ PartEncoder = (*DBConnectInfo)(nil)
)

// check if part types implement the right part decoder interface.
var (
	_ PartDecoder = (*HdbErrors)(nil)
	_ PartDecoder = (*AuthInitRequest)(nil)
	_ PartDecoder = (*AuthInitReply)(nil)
	_ PartDecoder = (*AuthFinalRequest)(nil)
	_ PartDecoder = (*AuthFinalReply)(nil)
	_ PartDecoder = (*ClientID)(nil)
	_ PartDecoder = (*clientInfo)(nil)
	_ PartDecoder = (*TopologyInformation)(nil)
	_ PartDecoder = (*Command)(nil)
	_ PartDecoder = (*RowsAffected)(nil)
	_ PartDecoder = (*StatementID)(nil)
	_ PartDecoder = (*ParameterMetadata)(nil)
	_ PartDecoder = (*InputParameters)(nil)
	_ PartDecoder = (*ResultMetadata)(nil)
	_ PartDecoder = (*ResultsetID)(nil)
	_ PartDecoder = (*Fetchsize)(nil)
	_ PartDecoder = (*ReadLobRequest)(nil)
	_ PartDecoder = (*WriteLobRequest)(nil)
	_ PartDecoder = (*ReadLobReply)(nil)
	_ PartDecoder = (*WriteLobReply)(nil)
	_ PartDecoder = (*ClientContext)(nil)
	_ PartDecoder = (*ConnectOptions)(nil)
	_ PartDecoder = (*DBConnectInfo)(nil)
	_ PartDecoder = (*statementContext)(nil)
	_ PartDecoder = (*transactionFlags)(nil)
)

// newPart instantiates the generic part decoder for kind. It returns false
// when a kind cannot be instantiated generically: authentication parts are
// bound to the auth handshake state, other non-generic parts need additional
// parameters (PkParameters, PkOutputParameters, PkResultset, PkReadLobReply
// - the latter needs the locator id from the originating request), and
// unknown kinds are ignored.
func newPart(kind PartKind) (PartDecoder, bool) {
	var part PartDecoder
	switch kind {
	case PkError:
		part = new(HdbErrors)
	case PkClientID:
		part = new(ClientID)
	case PkClientInfo:
		part = new(clientInfo)
	case PkTopologyInformation:
		part = new(TopologyInformation)
	case PkCommand:
		part = new(Command)
	case PkRowsAffected:
		part = new(RowsAffected)
	case PkStatementID:
		part = new(StatementID)
	case PkResultsetID:
		part = new(ResultsetID)
	case PkFetchSize:
		part = new(Fetchsize)
	case PkReadLobRequest:
		part = new(ReadLobRequest)
	case PkWriteLobReply:
		part = new(WriteLobReply)
	case PkWriteLobRequest:
		part = new(WriteLobRequest)
	case PkClientContext:
		part = new(ClientContext)
	case PkConnectOptions:
		part = new(ConnectOptions)
	case PkTransactionFlags:
		part = new(transactionFlags)
	case PkStatementContext:
		part = new(statementContext)
	case PkDBConnectInfo:
		part = new(DBConnectInfo)
	case PkParameterMetadata:
		part = new(ParameterMetadata)
	case PkResultMetadata:
		part = new(ResultMetadata)
	case PkAuthentication:
		return nil, false
	default:
		return nil, false
	}
	return part, true
}
