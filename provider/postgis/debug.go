package postgis

import (
	"os"
	"strings"
)

// debug determines weather extra debugging output is enabled.
// change debug to true to enable additional debugging output
// for this package
const debug = false

const (
	EnvSQLDebugName    = "TEGOLA_SQL_DEBUG"
	EnvSQLDebugLayer   = "LAYER_SQL"
	EnvSQLDebugExecute = "EXECUTE_SQL"
)

var (
	debugLayerSQL   bool
	debugExecuteSQL bool
)

func init() {
	debugLayerSQL = strings.Contains(os.Getenv(EnvSQLDebugName), EnvSQLDebugLayer)
	debugExecuteSQL = strings.Contains(os.Getenv(EnvSQLDebugName), EnvSQLDebugExecute)
}
