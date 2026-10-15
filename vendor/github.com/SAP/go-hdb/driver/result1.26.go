//go:build !go1.27

package driver

import (
	"context"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

// ReadLob used by protocol LobReader.
func (qr *queryResult) ReadLob(request *p.ReadLobRequest, reply *p.ReadLobReply) error {
	if qr.closed {
		return ErrScanOnClosedResultset
	}
	return qr.session.readLob(context.Background(), request, reply)
}

// ReadLob used by protocol LobReader.
func (cr *callResult) ReadLob(request *p.ReadLobRequest, reply *p.ReadLobReply) error {
	if cr.closed {
		return ErrScanOnClosedResultset
	}
	return cr.session.readLob(context.Background(), request, reply)
}

// convertCallResult converts the stored procedure scalar output parameters.
func convertCallResult(cr *callResult, scanArgs []any) error {
	return stdCallDB.db().QueryRow("", cr).Scan(scanArgs...)
}
