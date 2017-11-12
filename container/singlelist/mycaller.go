package list

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// MyCallerFileLine returns the FileLine of the caller of the function that called it :)
func MyCallerFileLine() string {

	// we get the callers as uintptrs - but we just need 1
	fpcs := make([]uintptr, 1)

	// skip 3 levels to get to the caller of whoever called Caller()
	n := runtime.Callers(3, fpcs)
	if n == 0 {
		return "n/a" // proper error her would be better
	}

	// get the info of the actual function that's in the pointer
	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return "n/a"
	}

	// return its name
	filename, line := fun.FileLine(fpcs[0] - 1)
	filename = filepath.Base(filename)
	return fmt.Sprintf("%v:%v", filename, line)
}
