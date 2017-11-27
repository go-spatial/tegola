package gpkg

import (
	"testing"

	"github.com/stretchr/testify"
)


def TestGetGpkgConnection(t *testing.T) {
	// Check that getting a connection once opens a new connection
	// Check that getting a connection more than once doesn't open a new connection
	// Check that getting a connection updates it's request timestamp.
	// Check that getting a second connection opens a new connection
	// Check that releasing all connections to a database doesn't close it unless the close age
	//	has been reached.
	// Check that sigint, sighup, sigquit result in all connections being closed.
}