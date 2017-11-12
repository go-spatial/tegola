// +build !gotrace

package gotrace

import "time"

var nilfn = func() {}

func Trace(_ string) func()                   { return nilfn }
func T() func()                               { return nilfn }
func Tracefn(fn func(_ time.Duration)) func() { return nilfn }
