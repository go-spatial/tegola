package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"golang.org/x/exp/slices"
)

// ClientContextOption represents a client context option.
type ClientContextOption int8

// ClientContextOption constants.
const (
	CcoClientVersion            ClientContextOption = 1
	CcoClientType               ClientContextOption = 2
	CcoClientApplicationProgram ClientContextOption = 3
)

// DBConnectInfoType represents a database connect info type.
type DBConnectInfoType int8

// DBConnectInfoType constants.
const (
	CiDatabaseName DBConnectInfoType = 1 // string
	CiHost         DBConnectInfoType = 2 // string
	CiPort         DBConnectInfoType = 3 // int4
	CiIsConnected  DBConnectInfoType = 4 // bool
)

type statementContextType int8

const (
	scStatementSequenceInfo statementContextType = 1
	scServerExecutionTime   statementContextType = 2
)

// transaction flags
type transactionFlagType int8

const (
	tfRolledback                      transactionFlagType = 0
	tfCommited                        transactionFlagType = 1
	tfNewIsolationLevel               transactionFlagType = 2
	tfDDLCommitmodeChanged            transactionFlagType = 3
	tfWriteTransactionStarted         transactionFlagType = 4
	tfNowriteTransactionStarted       transactionFlagType = 5
	tfSessionClosingTransactionError  transactionFlagType = 6
	tfSessionClosingTransactionErrror transactionFlagType = 7
	tfReadOnlyMode                    transactionFlagType = 8
)

// Options represents a generic option part.
type Options[K ~int8] map[K]any

func (ops Options[K]) String() string {
	s := []string{}
	for i, typ := range ops {
		s = append(s, fmt.Sprintf("%v: %v", K(i), typ))
	}
	slices.Sort(s)
	return fmt.Sprintf("%v", s)
}

func (ops Options[K]) size() int {
	size := 2 * len(ops) //option + type
	for _, v := range ops {
		ot := getOptType(v)
		size += ot.size(v)
	}
	return size
}

func (ops Options[K]) numArg() int { return len(ops) }

func (ops *Options[K]) decode(dec *encoding.Decoder, ph *PartHeader) error {
	*ops = Options[K]{} // no reuse of maps - create new one
	for i := 0; i < ph.numArg(); i++ {
		k := K(dec.Int8())
		tc := typeCode(dec.Byte())
		ot := tc.optType()
		(*ops)[k] = ot.decode(dec)
	}
	return dec.Error()
}

func (ops Options[K]) encode(enc *encoding.Encoder) error {
	for k, v := range ops {
		enc.Int8(int8(k))
		ot := getOptType(v)
		enc.Int8(int8(ot.typeCode()))
		ot.encode(enc, v)
	}
	return nil
}
