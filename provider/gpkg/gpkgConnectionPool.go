package gpkg

import (
	"database/sql"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/terranodo/tegola/util"
)

const (
	// How long after a database is last requested it should be considered for closing (in seconds).
	DEFAULT_CLOSE_AGE = 5 * 60
)

var closeAge int

func overrideCloseAge(newAge) {
	// This function is primarily for testing, so tests don't have to wait DEFAULT_CLOSE_AGE.
	closeAge = newAge
}

// Maps a gpkg filename to a GpkgConnectionPool
var gpkgPoolRegistry map[string]*GpkgConnectionPool
var gpkgPoolRegistryMutex sync.Mutex

func init() {
	closeAge = DEFAULT_CLOSE_AGE
	gpkgPoolRegistry = make(map[string]*GpkgConnectionPool, 10)

	// This sets up cleanup to close any open database connections when the program is killed.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		// Block until a signal is received.
		s := <-sigs
		util.CodeLogger.Infof("Signal received: %v", s)

		// Program is exiting, close db connections regardless of wait group status.
		// Don't unlock registry or connection pools, as we don't want any further use before exit.
		gpkgPoolRegistryMutex.Lock()
		for _, conn := range gpkgPoolRegistry {
			conn.mutex.Lock()
			if conn.db != nil {
				util.CodeLogger.Infof("Closing gpkg db at: %v", conn.filepath)
				conn.db.Close()
				conn.db = nil
			}
		}

		// Undo notify so signal has normal effect & resend.
		signal.Reset(syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)
		p, err := os.FindProcess(os.Getpid())
		if err == nil {
			p.Signal(s)
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
		util.CodeLogger.Infof("Opening gpkg at: %v", filepath)
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
	conn.lastRequested = time.Now().Unix()
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

	currentTime := time.Now().Unix()
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
				util.CodeLogger.Warnf("Invalid GpkgConnectionPool.shareCount for %v: %v",
					conn.filepath, conn.shareCount)
			}
			util.CodeLogger.Infof("Closing GPKG unused in %v seconds: %v", closeAge, conn.filepath)
			conn.db.Close()
			conn.db = nil
		}
	}
	conn.mutex.Unlock()
}
