// +build cgo

package gpkg

import (
	"database/sql"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/terranodo/tegola/internal/log"
)

const (
	// How long after a database is last requested it should be considered for closing (in nanoseconds).
	// 5 minutes
	DEFAULT_CLOSE_AGE = 5 * 60 * 1000000000
)

var (
	closeAge     int64 // nanoseconds
	resendSignal bool  // This can be set to false by tests to prevent a signal from being resent.

	// Maps a gpkg filename to a ConnectionPool
	poolRegistry      map[string]*ConnectionPool
	poolRegistryMutex sync.Mutex
)

func init() {
	closeAge = DEFAULT_CLOSE_AGE
	resendSignal = true
	poolRegistry = make(map[string]*ConnectionPool, 10)

	// This sets up cleanup to close any open database connections when the program is killed.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		// Block until a signal is received.
		s := <-sigs
		log.Infof("Signal received: %v", s)

		// Program is exiting, close db connections regardless of wait group status & reset registry.
		poolRegistryMutex.Lock()
		for filepath, conn := range poolRegistry {
			conn.mutex.Lock()
			if conn.db != nil {
				log.Infof("Closing gpkg db at: %v", conn.filepath)
				conn.db.Close()
				delete(poolRegistry, filepath)
			}
		}
		poolRegistryMutex.Unlock()

		// Undo notify so signal has normal effect & resend.
		signal.Reset(syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
		if resendSignal {
			p, err := os.FindProcess(os.Getpid())
			if err == nil {
				p.Signal(s)
			}
		}
	}()
}

type ConnectionPool struct {
	filepath      string
	db            *sql.DB
	lastRequested int64
	mutex         *sync.Mutex
	shareCount    int
}

func NewConnectionPool() (cp *ConnectionPool) {
	cp = new(ConnectionPool)
	cp.mutex = new(sync.Mutex)
	return cp
}

func GetConnection(filepath string) (db *sql.DB, err error) {
	// Lock registry to create new pool for this filepath if needed
	poolRegistryMutex.Lock()
	if poolRegistry[filepath] == nil {
		poolRegistry[filepath] = NewConnectionPool()
		poolRegistry[filepath].filepath = filepath
	}
	poolRegistryMutex.Unlock()

	conn := poolRegistry[filepath]
	// Lock pool to open database if needed
	conn.mutex.Lock()
	if conn.db == nil {
		// Open the database and stash the connection
		log.Infof("Opening gpkg at: %v", filepath)

		db, err = sql.Open("sqlite3", filepath)
		conn.db = db
		if err == nil {
			conn.shareCount++
		}

	} else {
		// Otherwise, just return the stashed connection
		db = conn.db
		conn.shareCount++
	}
	conn.lastRequested = time.Now().UnixNano()
	conn.mutex.Unlock()

	go runOldConnectionCheck()

	return db, err
}

func ReleaseConnection(filepath string) {
	poolRegistryMutex.Lock()
	conn := poolRegistry[filepath]
	poolRegistryMutex.Unlock()

	conn.mutex.Lock()
	conn.shareCount--
	conn.mutex.Unlock()
}

func runOldConnectionCheck() {
	go closeOldConnections()
}

func closeOldConnections() {
	// This ensures this routine will run again after completing
	defer runOldConnectionCheck()
	time.Sleep(time.Duration(closeAge) * time.Nanosecond)

	currentTime := time.Now().UnixNano()
	poolRegistryMutex.Lock()
	for _, conn := range poolRegistry {

		if currentTime-conn.lastRequested > closeAge {
			go func(conn *ConnectionPool) {
				closeConnection(conn)
			}(conn)
		}
	}
	poolRegistryMutex.Unlock()
}

func closeConnection(conn *ConnectionPool) {
	conn.mutex.Lock()
	if conn.db != nil {
		if conn.shareCount < 1 {
			if conn.shareCount < 0 {
				log.Warnf("Invalid ConnectionPool.shareCount for %v: %v",
					conn.filepath, conn.shareCount)
			}
			log.Infof("Closing GPKG unused in %f nanoseconds: %v", float32(closeAge)/1000000000.0, conn.filepath)
			conn.db.Close()
			conn.db = nil
		}
	}
	conn.mutex.Unlock()
}
