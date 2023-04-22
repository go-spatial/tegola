//go:build !go1.19
// +build !go1.19

// Delete after go1.18 is out of maintenance.

package driver

import (
	"sync/atomic"
)

// go1.19 downport

// An atomicBool is an atomic boolean value.
// The zero value is false.
type atomicBool struct {
	//_ noCopy
	v uint32
}

// Load atomically loads and returns the value stored in x.
func (x *atomicBool) Load() bool { return atomic.LoadUint32(&x.v) != 0 }

// Store atomically stores val into x.
func (x *atomicBool) Store(val bool) { atomic.StoreUint32(&x.v, b32(val)) }

// Swap atomically stores new into x and returns the previous value.
func (x *atomicBool) Swap(new bool) (old bool) { return atomic.SwapUint32(&x.v, b32(new)) != 0 }

// CompareAndSwap executes the compare-and-swap operation for the boolean value x.
func (x *atomicBool) CompareAndSwap(old, new bool) (swapped bool) {
	return atomic.CompareAndSwapUint32(&x.v, b32(old), b32(new))
}

// b32 returns a uint32 0 or 1 representing b.
func b32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}
