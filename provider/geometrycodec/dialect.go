package geometrycodec

import "github.com/alexeydott/tegola/internal/sqltoken"

// SQLDialect applies consistent lexical rules to registration probes and checks.
type SQLDialect struct{ scanner sqltoken.Dialect }

var (
	Legacy     = SQLDialect{sqltoken.Legacy}
	PostgreSQL = SQLDialect{sqltoken.PostgreSQL}
	MySQL      = SQLDialect{sqltoken.MySQL}
	SQLite     = SQLDialect{sqltoken.SQLite}
	HANA       = SQLDialect{sqltoken.HANA}
)
