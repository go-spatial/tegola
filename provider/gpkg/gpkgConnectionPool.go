package gpkg

import (
	"database/sql"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/terranodo/tegola/internal/log"
)

const (
	// How long after a database is last requested it should be considered for closing (in nanoseconds).
	// 5 minutes
	DEFAULT_CLOSE_AGE = 5 * 60 * 1000000000
)

var closeAge int64    // nanoseconds
var resendSignal bool // This can be set to false by tests to prevent a signal from being resent.

// Maps a gpkg filename to a GpkgConnectionPool
var gpkgPoolRegistry map[string]*GpkgConnectionPool
var gpkgPoolRegistryMutex sync.Mutex

func init() {
	closeAge = DEFAULT_CLOSE_AGE
	resendSignal = true
	gpkgPoolRegistry = make(map[string]*GpkgConnectionPool, 10)

	// This sets up cleanup to close any open database connections when the program is killed.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		// Block until a signal is received.
		s := <-sigs
		log.Info("Signal received: %v", s)

		// Program is exiting, close db connections regardless of wait group status & reset registry.
		gpkgPoolRegistryMutex.Lock()
		for filepath, conn := range gpkgPoolRegistry {
			conn.mutex.Lock()
			if conn.db != nil {
				log.Info("Closing gpkg db at: %v", conn.filepath)
				conn.db.Close()
				delete(gpkgPoolRegistry, filepath)
			}
		}
		gpkgPoolRegistryMutex.Unlock()

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

type GpkgConnectionPool struct {
	filepath      string
	db            *sql.DB
	lastRequested int64
	mutex         *sync.Mutex
	shareCount    int
}

func NewGpkgConnectionPool() (cp *GpkgConnectionPool) {
	cp = new(GpkgConnectionPool)
	cp.mutex = new(sync.Mutex)
	return cp
}

func getGpkgConnection(filepath string) (db *sql.DB, err error) {
	// Lock registry to create new pool for this filepath if needed
	gpkgPoolRegistryMutex.Lock()
	if gpkgPoolRegistry[filepath] == nil {
		gpkgPoolRegistry[filepath] = NewGpkgConnectionPool()
		gpkgPoolRegistry[filepath].filepath = filepath
	}
	gpkgPoolRegistryMutex.Unlock()

	conn := gpkgPoolRegistry[filepath]
	// Lock pool to open database if needed
	conn.mutex.Lock()
	if conn.db == nil {
		// Open the database and stash the connection
		log.Info("Opening gpkg at: %v", filepath)
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

func releaseGpkgConnection(filepath string) {
	gpkgPoolRegistryMutex.Lock()
	conn := gpkgPoolRegistry[filepath]
	gpkgPoolRegistryMutex.Unlock()

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
	gpkgPoolRegistryMutex.Lock()
	for _, conn := range gpkgPoolRegistry {

		if currentTime-conn.lastRequested > closeAge {
			go func(conn *GpkgConnectionPool) {
				closeConnection(conn)
			}(conn)
		}
	}
	gpkgPoolRegistryMutex.Unlock()
}

func closeConnection(conn *GpkgConnectionPool) {
	conn.mutex.Lock()
	if conn.db != nil {
		if conn.shareCount < 1 {
			if conn.shareCount < 0 {
				log.Warn("Invalid GpkgConnectionPool.shareCount for %v: %v",
					conn.filepath, conn.shareCount)
			}
			log.Info("Closing GPKG unused in %f nanoseconds: %v", float32(closeAge)/1000000000.0, conn.filepath)
			conn.db.Close()
			conn.db = nil
		}
	}
	conn.mutex.Unlock()
}
