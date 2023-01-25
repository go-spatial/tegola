package sqltrace

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/SAP/go-hdb/driver/internal/logflag"
)

// Trace is the sqlTrace logging instance.
var Trace = log.New(io.Discard, "hdb sql ", log.Ldate|log.Ltime)

var traceFlag = logflag.New(Trace)

func init() {
	flag.Var(traceFlag, "hdb.sqlTrace", "enabling hdb sql trace")
}

// On returns if tracing methods output is active.
func On() bool { return Trace.Writer() != io.Discard }

// SetOn sets tracing methods output active or inactive.
func SetOn(on bool) {
	if on {
		Trace.SetOutput(os.Stderr)
	} else {
		Trace.SetOutput(io.Discard)
	}
}
