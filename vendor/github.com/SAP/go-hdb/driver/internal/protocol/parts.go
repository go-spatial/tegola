package protocol

import (
	"fmt"
	"reflect"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

type partEncoder interface {
	size() int
	encode(*encoding.Encoder) error
}

type partDecoder interface {
	decode(*encoding.Decoder, *PartHeader) error
}

type part interface {
	String() string // should support Stringer interface
	kind() PartKind
}

func (*HdbErrors) kind() PartKind           { return PkError }
func (*AuthInitRequest) kind() PartKind     { return PkAuthentication }
func (*AuthInitReply) kind() PartKind       { return PkAuthentication }
func (*AuthFinalRequest) kind() PartKind    { return PkAuthentication }
func (*AuthFinalReply) kind() PartKind      { return PkAuthentication }
func (ClientID) kind() PartKind             { return PkClientID }
func (clientInfo) kind() PartKind           { return PkClientInfo }
func (*topologyInformation) kind() PartKind { return PkTopologyInformation }
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

var (
	typeOfClientContext    = reflect.TypeOf((*Options[ClientContextOption])(nil)).Elem()
	typeOfConnectOptions   = reflect.TypeOf((*Options[ConnectOption])(nil)).Elem()
	typeOfTransactionflags = reflect.TypeOf((*Options[transactionFlagType])(nil)).Elem()
	typeOfStatementContext = reflect.TypeOf((*Options[statementContextType])(nil)).Elem()
	typeOfDBConnectInfo    = reflect.TypeOf((*Options[DBConnectInfoType])(nil)).Elem()
)

func (ops Options[K]) kind() PartKind {
	switch reflect.TypeOf(ops) {
	case typeOfClientContext:
		return PkClientContext
	case typeOfConnectOptions:
		return PkConnectOptions
	case typeOfTransactionflags:
		return PkTransactionFlags
	case typeOfStatementContext:
		return PkStatementContext
	case typeOfDBConnectInfo:
		return PkDBConnectInfo
	default:
		panic("invalid options type") // should never happen
	}
}

type partWriter interface {
	part
	numArg() int
	partEncoder
}

// numArg methods (result == 1)
func (*AuthInitRequest) numArg() int  { return 1 }
func (*AuthFinalRequest) numArg() int { return 1 }
func (ClientID) numArg() int          { return 1 }
func (Command) numArg() int           { return 1 }
func (StatementID) numArg() int       { return 1 }
func (ResultsetID) numArg() int       { return 1 }
func (Fetchsize) numArg() int         { return 1 }
func (*ReadLobRequest) numArg() int   { return 1 }

// func (lobFlags) numArg() int                   { return 1 }

// size methods (fixed size)
const (
	statementIDSize    = 8
	resultsetIDSize    = 8
	fetchsizeSize      = 4
	readLobRequestSize = 24
)

func (StatementID) size() int    { return statementIDSize }
func (ResultsetID) size() int    { return resultsetIDSize }
func (Fetchsize) size() int      { return fetchsizeSize }
func (ReadLobRequest) size() int { return readLobRequestSize }

// func (lobFlags) size() int       { return tinyintFieldSize }

// check if part types implement partWriter interface
var (
	_ partWriter = (*AuthInitRequest)(nil)
	_ partWriter = (*AuthFinalRequest)(nil)
	_ partWriter = (*ClientID)(nil)
	_ partWriter = (*clientInfo)(nil)
	_ partWriter = (*Command)(nil)
	_ partWriter = (*StatementID)(nil)
	_ partWriter = (*InputParameters)(nil)
	_ partWriter = (*ResultsetID)(nil)
	_ partWriter = (*Fetchsize)(nil)
	_ partWriter = (*ReadLobRequest)(nil)
	_ partWriter = (*WriteLobRequest)(nil)
	_ partWriter = (*Options[ClientContextOption])(nil) // sufficient to check one option.
)

type partReader interface {
	part
	partDecoder
}

// check if part types implement partReader interface
var (
	_ partReader = (*HdbErrors)(nil)
	_ partReader = (*AuthInitRequest)(nil)
	_ partReader = (*AuthInitReply)(nil)
	_ partReader = (*AuthFinalRequest)(nil)
	_ partReader = (*AuthFinalReply)(nil)
	_ partReader = (*ClientID)(nil)
	_ partReader = (*clientInfo)(nil)
	_ partReader = (*topologyInformation)(nil)
	_ partReader = (*Command)(nil)
	_ partReader = (*RowsAffected)(nil)
	_ partReader = (*StatementID)(nil)
	_ partReader = (*ParameterMetadata)(nil)
	_ partReader = (*InputParameters)(nil)
	_ partReader = (*OutputParameters)(nil)
	_ partReader = (*ResultMetadata)(nil)
	_ partReader = (*ResultsetID)(nil)
	_ partReader = (*Resultset)(nil)
	_ partReader = (*Fetchsize)(nil)
	_ partReader = (*ReadLobRequest)(nil)
	_ partReader = (*WriteLobRequest)(nil)
	_ partReader = (*ReadLobReply)(nil)
	_ partReader = (*WriteLobReply)(nil)
	_ partReader = (*Options[ClientContextOption])(nil) // sufficient to check one option.
)

// some partReader needs additional parameter set before reading
type prmPartReader interface {
	partReader
	prm() // marker interface
}

// prm marker methods
func (*InputParameters) prm()  {}
func (*OutputParameters) prm() {}
func (*Resultset) prm()        {}

var (
	_ prmPartReader = (*InputParameters)(nil)
	_ prmPartReader = (*OutputParameters)(nil)
	_ prmPartReader = (*Resultset)(nil)
)

var partTypeMap = map[PartKind]reflect.Type{
	PkError:               reflect.TypeOf((*HdbErrors)(nil)).Elem(),
	PkClientID:            reflect.TypeOf((*ClientID)(nil)).Elem(),
	PkClientInfo:          reflect.TypeOf((*clientInfo)(nil)).Elem(),
	PkTopologyInformation: reflect.TypeOf((*topologyInformation)(nil)).Elem(),
	PkCommand:             reflect.TypeOf((*Command)(nil)).Elem(),
	PkRowsAffected:        reflect.TypeOf((*RowsAffected)(nil)).Elem(),
	PkStatementID:         reflect.TypeOf((*StatementID)(nil)).Elem(),
	PkParameterMetadata:   reflect.TypeOf((*ParameterMetadata)(nil)).Elem(),
	PkParameters:          reflect.TypeOf((*InputParameters)(nil)).Elem(),
	PkOutputParameters:    reflect.TypeOf((*OutputParameters)(nil)).Elem(),
	PkResultMetadata:      reflect.TypeOf((*ResultMetadata)(nil)).Elem(),
	PkResultsetID:         reflect.TypeOf((*ResultsetID)(nil)).Elem(),
	PkResultset:           reflect.TypeOf((*Resultset)(nil)).Elem(),
	PkFetchSize:           reflect.TypeOf((*Fetchsize)(nil)).Elem(),
	PkReadLobRequest:      reflect.TypeOf((*ReadLobRequest)(nil)).Elem(),
	PkReadLobReply:        reflect.TypeOf((*ReadLobReply)(nil)).Elem(),
	PkWriteLobReply:       reflect.TypeOf((*WriteLobReply)(nil)).Elem(),
	PkWriteLobRequest:     reflect.TypeOf((*WriteLobRequest)(nil)).Elem(),
	PkClientContext:       typeOfClientContext,
	PkConnectOptions:      typeOfConnectOptions,
	PkTransactionFlags:    typeOfTransactionflags,
	PkStatementContext:    typeOfStatementContext,
	PkDBConnectInfo:       typeOfDBConnectInfo,
}

func partType(pk PartKind) reflect.Type {
	if pt, ok := partTypeMap[pk]; ok {
		return pt
	}
	return nil
}

// newGenPartReader returns a generic part reader (part where no additional parameters are needed for reading it).
func newGenPartReader(pk PartKind) (partReader, bool) {
	pt := partType(pk)
	if pt == nil {
		// part kind is not (yet) supported by driver
		// need this in case server would send part kinds unknown to the driver
		return nil, false
	}

	// create instance
	part := reflect.New(pt).Interface()
	if _, ok := part.(prmPartReader); ok {
		// part kinds which do need additional parameters cannot be used
		return nil, false
	}
	partReader, ok := part.(partReader)
	if !ok {
		panic(fmt.Sprintf("part kind %s does not implement part reader interface", pk)) // should never happen
	}
	return partReader, true
}
