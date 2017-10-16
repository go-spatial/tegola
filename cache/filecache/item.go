package filecache

import "sync"

type Item struct {
	sync.RWMutex
}
