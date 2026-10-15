// Package audit provides shared SQL templates for W13 audit/outbox tables.
package audit

// SQLite (GPKG) DDL
const SQLiteDDL = `
CREATE TABLE IF NOT EXISTS tegola_audit (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts TEXT NOT NULL,
	actor TEXT NOT NULL DEFAULT '',
	collection TEXT NOT NULL,
	operation TEXT NOT NULL,
	feature_id INTEGER NOT NULL,
	revision_before TEXT NOT NULL DEFAULT '',
	revision_after TEXT NOT NULL DEFAULT '',
	transaction_id TEXT NOT NULL DEFAULT '',
	request_id TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS tegola_outbox (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts TEXT NOT NULL,
	event_type TEXT NOT NULL,
	collection TEXT NOT NULL,
	feature_id INTEGER NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	dispatched INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_tegola_outbox_undispatched ON tegola_outbox(dispatched) WHERE dispatched = 0;
CREATE TABLE IF NOT EXISTS tegola_revisions (
	collection TEXT NOT NULL,
	feature_id INTEGER NOT NULL,
	revision INTEGER NOT NULL DEFAULT 0,
	incarnation INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (collection, feature_id)
);
CREATE TABLE IF NOT EXISTS tegola_schema_version (
	version INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
);
`

// MySQL DDL keeps each indexed string below the legacy InnoDB 767-byte
// limit. Audit collection names retain 255 Unicode characters; their lookup
// index is non-unique. Revision collections are physical MySQL table names
// (at most 64 BMP characters), indexed in full with case-sensitive equality.
// CREATE IF NOT EXISTS intentionally leaves existing service schemas intact.
const MySQLDDL = `
CREATE TABLE IF NOT EXISTS tegola_audit (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	ts VARCHAR(40) NOT NULL,
	actor VARCHAR(255) NOT NULL DEFAULT '',
	collection VARCHAR(255) NOT NULL,
	operation VARCHAR(20) NOT NULL,
	feature_id BIGINT NOT NULL,
	revision_before TEXT,
	revision_after TEXT,
	transaction_id VARCHAR(100) DEFAULT '',
	request_id VARCHAR(100) DEFAULT '',
	INDEX idx_audit_collection (collection(191)),
	INDEX idx_audit_ts (ts)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tegola_outbox (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	ts VARCHAR(40) NOT NULL,
	event_type VARCHAR(50) NOT NULL,
	collection VARCHAR(255) NOT NULL,
	feature_id BIGINT NOT NULL,
	payload TEXT,
	dispatched TINYINT NOT NULL DEFAULT 0,
	INDEX idx_outbox_undispatched (dispatched)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tegola_revisions (
	collection VARCHAR(64) CHARACTER SET utf8 COLLATE utf8_bin NOT NULL,
	feature_id BIGINT NOT NULL,
	revision BIGINT NOT NULL DEFAULT 0,
	incarnation BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (collection, feature_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE TABLE IF NOT EXISTS tegola_schema_version (
	version INT PRIMARY KEY,
	applied_at VARCHAR(40) NOT NULL
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
`

// PostgreSQL DDL
const PostgresDDL = `
CREATE TABLE IF NOT EXISTS tegola_audit (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	actor TEXT NOT NULL DEFAULT '',
	collection TEXT NOT NULL,
	operation TEXT NOT NULL,
	feature_id BIGINT NOT NULL,
	revision_before TEXT DEFAULT '',
	revision_after TEXT DEFAULT '',
	transaction_id TEXT DEFAULT '',
	request_id TEXT DEFAULT ''
);
CREATE TABLE IF NOT EXISTS tegola_outbox (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	event_type TEXT NOT NULL,
	collection TEXT NOT NULL,
	feature_id BIGINT NOT NULL,
	payload JSONB DEFAULT '{}',
	dispatched BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_tegola_outbox_undispatched ON tegola_outbox(dispatched) WHERE NOT dispatched;
CREATE TABLE IF NOT EXISTS tegola_revisions (
	collection TEXT NOT NULL,
	feature_id BIGINT NOT NULL,
	revision BIGINT NOT NULL DEFAULT 0,
	incarnation BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (collection, feature_id)
);
CREATE TABLE IF NOT EXISTS tegola_schema_version (
	version INTEGER PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
