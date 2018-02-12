// +build cgo

package gpkg

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/terranodo/tegola/internal/assert"
)

var (
	GPKGAthensFilePath       = "test_data/athens-osm-20170921.gpkg"
	GPKGNaturalEarthFilePath = "test_data/natural_earth_minimal.gpkg"
	GPKGPuertoMontFilePath   = "test_data/puerto_mont-osm-20170922.gpkg"
)

func TestBasicConnectionPoolUse(t *testing.T) {
	// Checks that GetConnection, ReleaseConnection, and signal handling have the
	// expected effect on open connections.

	// Set close age to 100 milliseconds for testing purposes (closeAge uses nanoseconds)
	closeAge = 100 * 1000000

	// --- Check that getting a connection once opens a new connection
	assert.Equal(t, 0, len(poolRegistry))
	GetConnection(GPKGPuertoMontFilePath) // files[0] get #1
	assert.Equal(t, 1, len(poolRegistry))

	// --- Check that getting a connection more than once doesn't open a new connection
	GetConnection(GPKGPuertoMontFilePath) // files[0] get #2
	assert.Equal(t, 1, len(poolRegistry))

	// --- Check that getting a connection updates it's request timestamp.
	t1 := poolRegistry[GPKGPuertoMontFilePath].lastRequested
	GetConnection(GPKGPuertoMontFilePath) // files[0] get #3
	t2 := poolRegistry[GPKGPuertoMontFilePath].lastRequested

	assert.True(t, t2 > t1)

	// --- Check that getting a different connection opens a new connection
	GetConnection(GPKGAthensFilePath) // files[1] get #1
	assert.Equal(t, 2, len(poolRegistry))

	// --- Check that releasing all connections to a database closes it after close age has been reached
	ReleaseConnection(GPKGPuertoMontFilePath) // files[0] release #1
	ReleaseConnection(GPKGPuertoMontFilePath) // files[0] release #2
	ReleaseConnection(GPKGPuertoMontFilePath) // files[0] release #3
	assert.Equal(t, 0, poolRegistry[GPKGPuertoMontFilePath].shareCount)

	assert.NotNil(t, poolRegistry[GPKGPuertoMontFilePath].db)
	assert.NotNil(t, poolRegistry[GPKGAthensFilePath].db)

	// GPKGAthensFilePath hasn't been released, so should still be open after closeAge has passed.
	// Sleep for the closeAge plus some to allow the connection to age and the close to be processed.
	time.Sleep(time.Duration(closeAge*2) * time.Nanosecond)
	assert.Nil(t, poolRegistry[GPKGPuertoMontFilePath].db)
	assert.NotNil(t, poolRegistry[GPKGAthensFilePath].db)

	// --- Check that SIGINT results in registry reset.
	// TODO: Add mock to ensure that db.Close() is called for each connection.
	GetConnection(GPKGPuertoMontFilePath)
	assert.NotNil(t, poolRegistry[GPKGPuertoMontFilePath].db)
	assert.NotNil(t, poolRegistry[GPKGAthensFilePath].db)
	// This instructs the connection pool cleanup code not to resend the signal and exit the process
	resendSignal = false
	p, err := os.FindProcess(os.Getpid())
	assert.Nil(t, err)
	p.Signal(syscall.SIGINT)

	// Sleep briefly to allow cleanup code time to complete
	time.Sleep(time.Duration(25) * time.Millisecond)
	assert.Equal(t, 0, len(poolRegistry))
}
