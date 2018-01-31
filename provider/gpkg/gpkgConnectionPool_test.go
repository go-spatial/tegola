package gpkg

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/terranodo/tegola/internal/assert"
)

var testDataPath string

func init() {
	_, filePath, _, _ = runtime.Caller(0)
	directory, _ = filepath.Split(filePath)
	testDataPath, _ = filepath.Split(filePath)
	testDataPath = testDataPath + "test_data/"
}

var gpkgFiles []string = []string{
	testDataPath + "puerto_mont-osm-20170922.gpkg",
	testDataPath + "athens-osm-20170921.gpkg",
}

func TestBasicConnectionPoolUse(t *testing.T) {
	// Checks that GetGpkgConnection, ReleaseGpkgConnection, and signal handling have the
	//	expected effect on open connections.

	// Set close age to 100 milliseconds for testing purposes (closeAge uses nanoseconds)
	closeAge = 100 * 1000000

	// --- Check that getting a connection once opens a new connection
	assert.Equal(t, 0, len(gpkgPoolRegistry))
	GetGpkgConnection(gpkgFiles[0]) // files[0] get #1
	assert.Equal(t, 1, len(gpkgPoolRegistry))

	// --- Check that getting a connection more than once doesn't open a new connection
	GetGpkgConnection(gpkgFiles[0]) // files[0] get #2
	assert.Equal(t, 1, len(gpkgPoolRegistry))

	// --- Check that getting a connection updates it's request timestamp.
	t1 := gpkgPoolRegistry[gpkgFiles[0]].lastRequested
	GetGpkgConnection(gpkgFiles[0]) // files[0] get #3
	t2 := gpkgPoolRegistry[gpkgFiles[0]].lastRequested

	assert.True(t, t2 > t1)

	// --- Check that getting a different connection opens a new connection
	GetGpkgConnection(gpkgFiles[1]) // files[1] get #1
	assert.Equal(t, 2, len(gpkgPoolRegistry))

	// --- Check that releasing all connections to a database closes it after close age has been reached
	ReleaseGpkgConnection(gpkgFiles[0]) // files[0] release #1
	ReleaseGpkgConnection(gpkgFiles[0]) // files[0] release #2
	ReleaseGpkgConnection(gpkgFiles[0]) // files[0] release #3
	assert.Equal(t, 0, gpkgPoolRegistry[gpkgFiles[0]].shareCount)

	assert.NotNil(t, gpkgPoolRegistry[gpkgFiles[0]].db)
	assert.NotNil(t, gpkgPoolRegistry[gpkgFiles[1]].db)

	// gpkgFiles[1] hasn't been released, so should still be open after closeAge has passed.
	// Sleep for the closeAge plus some to allow the connection to age and the close to be processed.
	time.Sleep(time.Duration(closeAge*2) * time.Nanosecond)
	assert.Nil(t, gpkgPoolRegistry[gpkgFiles[0]].db)
	assert.NotNil(t, gpkgPoolRegistry[gpkgFiles[1]].db)

	// --- Check that SIGINT results in registry reset.
	// TODO: Add mock to ensure that db.Close() is called for each connection.
	GetGpkgConnection(gpkgFiles[0])
	assert.NotNil(t, gpkgPoolRegistry[gpkgFiles[0]].db)
	assert.NotNil(t, gpkgPoolRegistry[gpkgFiles[1]].db)
	// This instructs the connection pool cleanup code not to resend the signal and exit the process
	resendSignal = false
	p, err := os.FindProcess(os.Getpid())
	assert.Nil(t, err)
	p.Signal(syscall.SIGINT)

	// Sleep briefly to allow cleanup code time to complete
	time.Sleep(time.Duration(25) * time.Millisecond)
	assert.Equal(t, 0, len(gpkgPoolRegistry))
}
