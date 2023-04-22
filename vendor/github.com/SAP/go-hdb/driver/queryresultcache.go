package driver

import (
	"database/sql/driver"
	"sync"
)

// queryResultCache is a query result cache supporting reading
// procedure (call) table parameter via separate query (legacy mode).
var stdQueryResultCache = newQueryResultCache()

type queryResultCache struct {
	cache map[uint64]*queryResult
	mu    sync.RWMutex
}

func newQueryResultCache() *queryResultCache {
	return &queryResultCache{cache: map[uint64]*queryResult{}}
}

func (c *queryResultCache) set(id uint64, qr *queryResult) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[id] = qr
	return id
}

func (c *queryResultCache) Get(id uint64) (driver.Rows, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	qr, ok := c.cache[id]
	return qr, ok
}

func (c *queryResultCache) cleanup(conn *conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, qr := range c.cache {
		if qr.conn == conn {
			delete(c.cache, id)
		}
	}
}
