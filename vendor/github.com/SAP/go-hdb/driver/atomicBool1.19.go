//go:build go1.19
// +build go1.19

// Delete after go1.17 is out of maintenance.

package driver

import (
	"sync/atomic"
)

// aliase
type atomicBool = atomic.Bool
