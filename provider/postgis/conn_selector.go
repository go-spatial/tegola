package postgis

import "os"

// ConnMode is the stragey used to establish a connection
// to a PostGIS database.
type ConnMode string

const (
	// connModeEnv indicates that the connection coonfiguration
	// should be derived from the environment.
	connModeEnv ConnMode = "env"
	// connModeUri indicates that the connection configuration
	// should be derived from the uri provided in the configuration.
	connModeURI ConnMode = "uri"
)

// envTriggers are those env variables that trigger ConnMode to be env.
// in which case environment variables are the prefered way of connecting
// to PostGIS.
//
// The focus is on environment variables that a connection relevant.
// See https://www.postgresql.org/docs/10/libpq-envars.html
// for an exhaustive list.
var envTriggers = []string{
	"PGHOST",
	"PGPORT",
	"PGDATABASE",
	"PGUSER",
	"PGPASSWORD",
	"PGSSLMODE",
	"PGSSLCERT",
	"PGSSLKEY",
	"PGSSLROOTCERT",
}

// selector defines the behavior required to determine
// which connection mode should be used.
type selector interface {
	Select()
}

// envSelector implements selector by inspecting environment variables.
type envSelector struct{}

// Select determines the connection mode based on the presense of
// libpq-related environment variables.
//
// If any of the variables in envTriggers is non-empty we assume
// the connModeEnv, otherwise connModeURI where we expect the presence
// of a config connection URI.
func (e envSelector) Select() ConnMode {
	for _, env := range envTriggers {
		if value := os.Getenv(env); value != "" {
			return connModeEnv
		}
	}

	return connModeURI
}
