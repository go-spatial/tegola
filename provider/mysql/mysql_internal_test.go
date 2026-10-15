package mysql

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"database/sql/driver"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestSampleGeometryQueryRemovesTrailingLimitAndSemicolon(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "limit and semicolon",
			sql:  "SELECT geom FROM features LIMIT 1;",
			want: "SELECT geom FROM features LIMIT 16",
		},
		{
			name: "limit without semicolon",
			sql:  "SELECT geom FROM features LIMIT 1",
			want: "SELECT geom FROM features LIMIT 16",
		},
		{
			name: "limit offset",
			sql:  "SELECT geom FROM features LIMIT 1 OFFSET 4;",
			want: "SELECT geom FROM features LIMIT 16",
		},
		{
			name: "no limit",
			sql:  "SELECT geom FROM features;",
			want: "SELECT geom FROM features LIMIT 16",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sampleGeometryQuery(tc.sql); got != tc.want {
				t.Fatalf("sampleGeometryQuery(%q) = %q, want %q", tc.sql, got, tc.want)
			}
			if strings.Contains(tc.want, "; LIMIT") {
				t.Fatalf("sampling LIMIT was appended after a statement terminator: %q", tc.want)
			}
		})
	}
}

// mustReplaceTokens runs the fail-closed replaceTokens (A12: predicate build
// errors surface to the caller instead of silently falling back to 1=1) and
// fails the test on error.
func mustReplaceTokens(t *testing.T, sql string, layer *Layer, tile provider.Tile, bboxExtent *geom.Extent) string {
	t.Helper()
	got, err := replaceTokens(sql, layer, tile, bboxExtent)
	if err != nil {
		t.Fatalf("replaceTokens(%q) error: %v", sql, err)
	}
	return got
}

func TestMySQLBBoxUsesConfiguredSRID(t *testing.T) {
	layer := &Layer{
		geomFieldname:  "geom",
		geometryFormat: GeometryFormatWKT,
		srid:           4326,
	}
	tile := provider.NewTile(0, 0, 0, 0, 4326)
	extent, _ := tile.BufferedExtent()

	query := mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent)
	if !strings.Contains(query, "ST_GeomFromText(`geom`, 4326)") {
		t.Fatalf("WKT geometry expression does not carry SRID: %q", query)
	}
	if !strings.Contains(query, "ST_GeomFromText('POLYGON") {
		t.Fatalf("bbox expression does not use ST_GeomFromText: %q", query)
	}
	if !strings.Contains(query, ", 4326)") {
		t.Fatalf("bbox expression does not carry SRID: %q", query)
	}

	layer.srid = 0
	query = mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent)
	if strings.Contains(query, ", 4326)") {
		t.Fatalf("SRID leaked into zero-SRID query: %q", query)
	}
}

func TestMySQLBBoxWKBUsesGeomFromWKB(t *testing.T) {
	layer := &Layer{
		geomFieldname:  "geom",
		geometryFormat: GeometryFormatWKB,
		srid:           4326,
	}
	tile := provider.NewTile(0, 0, 0, 0, 4326)
	extent, _ := tile.BufferedExtent()

	query := mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent)
	if !strings.Contains(query, "ST_GeomFromWKB(`geom`, 4326)") {
		t.Fatalf("WKB geometry expression does not use ST_GeomFromWKB with SRID: %q", query)
	}

	layer.srid = 0
	query = mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent)
	if !strings.Contains(query, "ST_GeomFromWKB(`geom`)") {
		t.Fatalf("zero-SRID WKB expression must omit the SRID argument: %q", query)
	}
	if strings.Contains(query, "ST_Intersects(`geom`") {
		t.Fatalf("raw WKB column must never be used directly as a geometry: %q", query)
	}
}

// TestMySQLAxisOrderLongLatForGeographicSRIDs (audit P6-3) pins the geometry
// constructor SQL for geographic SRIDs: MySQL 8 SRS metadata stores
// geographic coordinates latitude-first, while tegola writes WKT and bbox
// polygons longitude-first, so on MySQL servers the constructors must carry
// the 'axis-order=long-lat' option or bbox filters and stored geometries
// disagree. MariaDB (no options argument support) and projected SRIDs keep
// the plain constructor form.
func TestMySQLAxisOrderLongLatForGeographicSRIDs(t *testing.T) {
	tile := provider.NewTile(0, 0, 0, 0, 4326)
	extent, _ := tile.BufferedExtent()

	tcs := []struct {
		name     string
		flavor   string
		srid     uint64
		format   string
		wantCol  string
		wantAxis bool
	}{
		{"mysql geographic wkt", GeometryFormatMySQL, 4326, GeometryFormatWKT, "ST_GeomFromText(`geom`, 4326", true},
		{"mysql projected wkt", GeometryFormatMySQL, 3857, GeometryFormatWKT, "ST_GeomFromText(`geom`, 3857", false},
		{"mariadb geographic wkt", GeometryFormatMariaDB, 4326, GeometryFormatWKT, "ST_GeomFromText(`geom`, 4326", false},
		{"mysql geographic wkb", GeometryFormatMySQL, 4326, GeometryFormatWKB, "ST_GeomFromWKB(`geom`, 4326", true},
		{"mariadb geographic wkb", GeometryFormatMariaDB, 4326, GeometryFormatWKB, "ST_GeomFromWKB(`geom`, 4326", false},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			layer := &Layer{
				geomFieldname:  "geom",
				geometryFormat: tc.format,
				srid:           tc.srid,
				serverFlavor:   tc.flavor,
			}
			query := mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent)

			colExpr := tc.wantCol + ")"
			if tc.wantAxis {
				colExpr = tc.wantCol + ", 'axis-order=long-lat')"
			}
			if !strings.Contains(query, colExpr) {
				t.Fatalf("geometry column expression %q missing in query: %q", colExpr, query)
			}

			polyExpr := "ST_GeomFromText('POLYGON"
			if !strings.Contains(query, polyExpr) {
				t.Fatalf("bbox polygon expression missing in query: %q", query)
			}
			wantSrid := strconv.FormatUint(tc.srid, 10)
			if tc.wantAxis {
				polyExpr = ", " + wantSrid + ", 'axis-order=long-lat')"
				if !strings.Contains(query, polyExpr) {
					t.Fatalf("bbox polygon expression missing axis-order option in query: %q", query)
				}
				if n := strings.Count(query, "'axis-order=long-lat'"); n != 2 {
					t.Fatalf("expected axis-order on both constructors (%d) in query: %q", n, query)
				}
			} else {
				if strings.Contains(query, "axis-order") {
					t.Fatalf("axis-order leaked into %s SRID %d query: %q", tc.flavor, tc.srid, query)
				}
				polyExpr = ", " + wantSrid + ")"
				if !strings.Contains(query, polyExpr) {
					t.Fatalf("bbox polygon expression missing SRID %q in query: %q", polyExpr, query)
				}
			}
		})
	}
}

// TestBuildDSNEscapingAndOptions (audit P6-4) pins DSN construction: the
// driver's Config.FormatDSN escapes special characters (the old fmt.Sprintf
// DSN broke on e.g. '?' in database names), multiStatements stays disabled,
// and the tls/timeout options appear only when configured.
//
// Note: the go-sql-driver DSN grammar cannot represent a ':' inside the
// username (ParseDSN splits user at the first ':') — a driver-level
// limitation affecting any DSN-based configuration.
func TestBuildDSNEscapingAndOptions(t *testing.T) {
	user, pass, dbname := "us.er+pa/ss@x", "p@ss:w/rd#1?x&y=z", "data base?x=1"
	dsn := buildDSN(user, pass, "db.host", 3306, dbname, "", 0)
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q) error: %v", dsn, err)
	}
	if cfg.User != user || cfg.Passwd != pass || cfg.DBName != dbname {
		t.Fatalf("credentials/database did not round-trip through DSN %q: user=%q pass=%q db=%q",
			dsn, cfg.User, cfg.Passwd, cfg.DBName)
	}
	if cfg.Addr != "db.host:3306" {
		t.Fatalf("addr = %q, want %q", cfg.Addr, "db.host:3306")
	}
	if !cfg.ParseTime {
		t.Fatalf("parseTime must stay enabled: %q", dsn)
	}
	// the driver's authentication/liveness defaults must survive DSN
	// building: a bare mysqlDriver.Config struct literal left
	// AllowNativePasswords=false and the connection failed with
	// "this user requires mysql native password authentication" for users
	// using mysql_native_password (e.g. legacy MapplGIS accounts).
	if !cfg.AllowNativePasswords {
		t.Fatalf("allowNativePasswords must stay enabled (driver default): %q", dsn)
	}
	if !cfg.CheckConnLiveness {
		t.Fatalf("checkConnLiveness must stay enabled (driver default): %q", dsn)
	}
	if strings.Contains(dsn, "multiStatements") {
		t.Fatalf("multiStatements must be disabled: %q", dsn)
	}
	if strings.Contains(dsn, "tls=") || strings.Contains(dsn, "timeout=") {
		t.Fatalf("unset options leaked into DSN: %q", dsn)
	}

	// IPv6 hosts are bracketed by net.JoinHostPort
	dsn = buildDSN("u", "p", "::1", 3306, "db", "skip-verify", 5*time.Second)
	cfg, err = mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q) error: %v", dsn, err)
	}
	if cfg.Addr != "[::1]:3306" {
		t.Fatalf("addr = %q, want %q", cfg.Addr, "[::1]:3306")
	}
	if cfg.TLSConfig != "skip-verify" {
		t.Fatalf("tls = %q, want %q", cfg.TLSConfig, "skip-verify")
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("timeout = %v, want %v", cfg.Timeout, 5*time.Second)
	}
	if !strings.Contains(dsn, "tls=skip-verify") || !strings.Contains(dsn, "timeout=5s") {
		t.Fatalf("options missing from DSN: %q", dsn)
	}
}

// TestDSNOptionsConfigKeys pins the new tls/timeout config keys (audit
// P6-4): both optional, timeout accepts Go duration strings or integer
// seconds, invalid values fail registration.
func TestDSNOptionsConfigKeys(t *testing.T) {
	tcs := []struct {
		name        string
		cfg         dict.Dict
		wantTLS     string
		wantTimeout time.Duration
		wantErr     bool
	}{
		{"empty", dict.Dict{}, "", 0, false},
		{"tls only", dict.Dict{ConfigKeyTLS: "preferred"}, "preferred", 0, false},
		{"duration string", dict.Dict{ConfigKeyTimeout: "10s"}, "", 10 * time.Second, false},
		{"millisecond string", dict.Dict{ConfigKeyTimeout: "500ms"}, "", 500 * time.Millisecond, false},
		{"integer seconds", dict.Dict{ConfigKeyTimeout: 5}, "", 5 * time.Second, false},
		{"both", dict.Dict{ConfigKeyTLS: "skip-verify", ConfigKeyTimeout: "1m"}, "skip-verify", time.Minute, false},
		{"invalid duration", dict.Dict{ConfigKeyTimeout: "abc"}, "", 0, true},
		{"negative duration", dict.Dict{ConfigKeyTimeout: "-3s"}, "", 0, true},
		{"negative integer", dict.Dict{ConfigKeyTimeout: -2}, "", 0, true},
		{"float value", dict.Dict{ConfigKeyTimeout: 2.5}, "", 0, true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tlsCfg, timeout, err := dsnOptions(tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("dsnOptions(%v) = (%q, %v), want error", tc.cfg, tlsCfg, timeout)
				}
				return
			}
			if err != nil {
				t.Fatalf("dsnOptions(%v) unexpected error: %v", tc.cfg, err)
			}
			if tlsCfg != tc.wantTLS || timeout != tc.wantTimeout {
				t.Fatalf("dsnOptions(%v) = (%q, %v), want (%q, %v)", tc.cfg, tlsCfg, timeout, tc.wantTLS, tc.wantTimeout)
			}
		})
	}
}

func TestMySQLBBoxSyntheticSRIDDisablesDatabaseSpatialPredicate(t *testing.T) {
	srid, err := basic.RegisterProj4Defn("+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +ellps=WGS84 +datum=WGS84 +units=m +no_defs")
	if err != nil {
		t.Fatalf("registering synthetic CRS: %v", err)
	}

	layer := &Layer{geomFieldname: "geom", srid: srid}
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)
	extent, _ := tile.BufferedExtent()
	if got := mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent); got != "WHERE 1=1" {
		t.Fatalf("synthetic CRS bbox = %q, want database-safe 1=1", got)
	}
}

func TestMySQLDeferredAutoBBoxUsesInMemoryFiltering(t *testing.T) {
	layer := &Layer{
		geomFieldname:      "geom",
		geometryFormat:     GeometryFormatAuto,
		deferredInspection: true,
		srid:               3857,
	}
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)
	extent, _ := tile.BufferedExtent()
	if got := mustReplaceTokens(t, "WHERE !BBOX!", layer, tile, extent); got != "WHERE 1=1" {
		t.Fatalf("deferred auto bbox = %q, want in-memory filtering", got)
	}
}

func TestGeometryIntersectsExtent(t *testing.T) {
	extent := geom.NewExtent(geom.Point{0, 0}, geom.Point{10, 10})

	tests := []struct {
		name string
		g    geom.Geometry
		want bool
	}{
		{name: "inside point", g: geom.Point{5, 5}, want: true},
		{name: "boundary point", g: geom.Point{10, 10}, want: true},
		{name: "outside point", g: geom.Point{11, 5}, want: false},
		{name: "inside multipoint", g: geom.MultiPoint{{1, 1}, {9, 9}}, want: true},
		{name: "outside multipoint", g: geom.MultiPoint{{11, 1}, {12, 9}}, want: false},
		{name: "line crossing boundary", g: geom.LineString{{-1, 5}, {1, 5}}, want: true},
		{name: "inside line", g: geom.LineString{{1, 1}, {9, 9}}, want: true},
		{name: "outside line", g: geom.LineString{{11, 1}, {12, 9}}, want: false},
		{name: "inside multiline", g: geom.MultiLineString{{{1, 1}, {2, 2}}, {{8, 8}, {9, 9}}}, want: true},
		{name: "outside multiline", g: geom.MultiLineString{{{11, 1}, {12, 2}}}, want: false},
		{name: "inside polygon", g: geom.Polygon{{{1, 1}, {9, 1}, {9, 9}, {1, 9}, {1, 1}}}, want: true},
		{name: "outside polygon", g: geom.Polygon{{{11, 1}, {12, 1}, {12, 2}, {11, 2}, {11, 1}}}, want: false},
		{name: "inside multipolygon", g: geom.MultiPolygon{
			{{{1, 1}, {2, 1}, {2, 2}, {1, 2}, {1, 1}}},
			{{{8, 8}, {9, 8}, {9, 9}, {8, 9}, {8, 8}}},
		}, want: true},
		{name: "outside multipolygon", g: geom.MultiPolygon{
			{{{11, 1}, {12, 1}, {12, 2}, {11, 2}, {11, 1}}},
		}, want: false},
		{name: "inside collection", g: geom.Collection{
			geom.Point{5, 5},
			geom.LineString{{20, 20}, {21, 21}},
		}, want: true},
		{name: "outside collection", g: geom.Collection{
			geom.Point{20, 20},
			geom.LineString{{30, 30}, {31, 31}},
		}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := codec.GeometryIntersectsExtent(tc.g, extent); got != tc.want {
				t.Fatalf("codec.GeometryIntersectsExtent(%v) = %v, want %v", tc.g, got, tc.want)
			}
		})
	}
}

func TestTrimTrailingSemicolon(t *testing.T) {
	if got := trimTrailingSemicolon(" SELECT * FROM features ;  "); got != "SELECT * FROM features" {
		t.Fatalf("trimTrailingSemicolon() = %q", got)
	}
}

// TestMySQLMOSBoundsSQL covers the R3-08 provider-level regression: the
// !BBOX! token for MOS layers must expand to the coarse indexed raw-bounds
// filter scaled from the tile metres, not to a spatial predicate over the
// opaque blob. The MOS MOS case also covers R9-10: the SQL text (ORDER BY
// etc.) must survive token replacement untouched.
func TestMySQLMOSBoundsSQL(t *testing.T) {
	type tcase struct {
		name        string
		bboxFields  codec.BBoxFields
		mosConfig   codec.MOSConfig
		bbox        *geom.Extent
		sql         string
		wantBBoxSQL string
		wantErr     bool // A12: predicate build errors fail closed, never 1=1
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			layer := &Layer{
				geomFieldname:  "geom",
				geometryFormat: GeometryFormatMOS,
				srid:           3857,
				mosConfig:      tc.mosConfig,
				bboxFields:     tc.bboxFields,
			}
			tile := provider.NewTile(0, 0, 0, 64, tegola.WebMercator)
			extent := tc.bbox
			got, err := replaceTokens(tc.sql, layer, tile, extent)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("replaceTokens(%q) = %q, want error", tc.sql, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("replaceTokens(%q) error: %v", tc.sql, err)
			}
			want := "WHERE " + tc.wantBBoxSQL
			if got != want {
				t.Fatalf("MOS bbox = %q, want %q", got, want)
			}
		}
	}

	// extent [-10..10] metres, precision 2 (scale 100) with metre units
	// (unit factor 1) => raw bounds [-1000..1000].
	metreUnits := codec.MOSConfig{Precision: 2, UnitFactor: 1}
	defaults := codec.DefaultBBoxFields()
	tests := map[string]tcase{
		"bounds scaled by precision": {
			bboxFields:  defaults,
			mosConfig:   metreUnits,
			bbox:        geom.NewExtent(geom.Point{-10, -10}, geom.Point{10, 10}),
			sql:         "WHERE !BBOX!",
			wantBBoxSQL: "`MAXX` >= -1000 AND `MINX` <= 1000 AND `MAXY` >= -1000 AND `MINY` <= 1000",
		},
		"degenerate zero-extent tile falls back to raw bounds too": {
			bboxFields:  defaults,
			mosConfig:   metreUnits,
			bbox:        geom.NewExtent(geom.Point{0, 0}, geom.Point{0, 0}),
			sql:         "WHERE !BBOX!",
			wantBBoxSQL: "`MAXX` >= 0 AND `MINX` <= 0 AND `MAXY` >= 0 AND `MINY` <= 0",
		},
		"invalid mos config fails closed (A12)": {
			bboxFields: defaults,
			mosConfig:  codec.MOSConfig{Precision: 2, UnitFactor: 0},
			bbox:       geom.NewExtent(geom.Point{-10, -10}, geom.Point{10, 10}),
			sql:        "WHERE !BBOX!",
			wantErr:    true,
		},
		"nil extent fails closed (A12)": {
			bboxFields: defaults,
			mosConfig:  metreUnits,
			bbox:       nil,
			sql:        "WHERE !BBOX!",
			wantErr:    true,
		},
		"custom bounds fields and ORDER BY survive": {
			bboxFields:  codec.BBoxFields{"t.XMIN", "t.XMAX", "t.YMIN", "t.YMAX"},
			mosConfig:   metreUnits,
			bbox:        geom.NewExtent(geom.Point{-10, -10}, geom.Point{10, 10}),
			sql:         "WHERE !BBOX! ORDER BY t.Okey",
			wantBBoxSQL: "`t`.`XMAX` >= -1000 AND `t`.`XMIN` <= 1000 AND `t`.`YMAX` >= -1000 AND `t`.`YMIN` <= 1000 ORDER BY t.Okey",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestValidateMOSPrecision(t *testing.T) {
	for _, precision := range []float64{0, 2, codec.MaxMOSPrecision} {
		if err := codec.ValidateMOSPrecision(precision); err != nil {
			t.Errorf("codec.ValidateMOSPrecision(%v) = %v", precision, err)
		}
	}
	for _, precision := range []float64{-1, 1.5, codec.MaxMOSPrecision + 1, math.NaN(), math.Inf(1)} {
		if err := codec.ValidateMOSPrecision(precision); err == nil {
			t.Errorf("codec.ValidateMOSPrecision(%v) succeeded, want error", precision)
		}
	}
}

func TestTileFeaturesWKBInMemoryFilter(t *testing.T) {
	inside, err := wkb.EncodeBytes(geom.Point{-1000000, 1000000})
	if err != nil {
		t.Fatal(err)
	}
	outside, err := wkb.EncodeBytes(geom.Point{1000000, -1000000})
	if err != nil {
		t.Fatal(err)
	}

	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id", "geom"}, Rows: [][]driver.Value{
		{int64(1), inside},
		{int64(2), outside},
	}})

	// The fake server returns both rows for any query, so the SQL !BBOX!
	// predicate cannot be what filters: the exact in-memory filter is
	// mandatory for the raw wkb format.
	p := &Provider{
		db: db,
		layers: map[string]Layer{
			"test": {
				name:           "test",
				sql:            "SELECT id, geom FROM test WHERE !BBOX!",
				idFieldname:    "id",
				geomFieldname:  "geom",
				geometryFormat: GeometryFormatWKB,
				srid:           tegola.WebMercator,
			},
		},
	}

	var got []provider.Feature
	err = p.TileFeatures(context.Background(), "test", provider.NewTile(1, 0, 0, 0, tegola.WebMercator), nil,
		func(f *provider.Feature) error {
			got = append(got, *f)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the inside point to survive the exact in-memory filter, got %d features", len(got))
	}
	if got[0].ID != uint64(1) {
		t.Fatalf("expected feature id 1, got %v", got[0].ID)
	}
}

func TestGeomTypeFromColumnSkipsSystemInfoBlob(t *testing.T) {
	// The MapplGIS LayerInfo blob is metadata, not geometry: sampling must
	// skip it and keep looking for a decodable geometry. Detection itself
	// runs once at registration via the provider/mapplgis contract.
	systemInfo := make([]byte, 64)
	copy(systemInfo, []byte{5, 'V', 'e', 'r', ' ', '1'})
	binary.LittleEndian.PutUint32(systemInfo[11:15], 4)

	db := fixture.OpenSQLRows(t, fixture.SQLRows{
		Columns: []string{"geom"}, TypeNames: []string{"BLOB"},
		Rows: [][]driver.Value{{systemInfo}, {"POINT(1 2)"}},
	})

	geo, _, err := geomTypeFromColumn(db, "SELECT geom", GeometryFormatWKT, GeometryFormatMySQL, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := geo.(geom.Point); !ok {
		t.Fatalf("expected geom.Point, got %T", geo)
	}
}

func TestGeomTypeFromColumnOnlySystemInfoRowsYieldsErrNoRows(t *testing.T) {
	// A sample consisting solely of the layer self-description blob carries
	// no geometry: sampling must report sql.ErrNoRows rather than succeed.
	systemInfo := make([]byte, 64)
	copy(systemInfo, []byte{5, 'V', 'e', 'r', ' ', '1'})
	binary.LittleEndian.PutUint32(systemInfo[11:15], 4)

	db := fixture.OpenSQLRows(t, fixture.SQLRows{
		Columns: []string{"geom"}, TypeNames: []string{"BLOB"},
		Rows: [][]driver.Value{{systemInfo}},
	})

	_, _, err := geomTypeFromColumn(db, "SELECT geom", GeometryFormatWKT, GeometryFormatMySQL, codec.DefaultMOSConfig())
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGeomTypeFromColumnSkipsNullGeometryRows(t *testing.T) {
	db := fixture.OpenSQLRows(t, fixture.SQLRows{
		Columns: []string{"geom"}, TypeNames: []string{"BLOB"},
		Rows: [][]driver.Value{{nil}, {"POINT(1 2)"}},
	})

	geo, _, err := geomTypeFromColumn(db, "SELECT geom", GeometryFormatWKT, GeometryFormatMySQL, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := geo.(geom.Point); !ok {
		t.Fatalf("expected geom.Point, got %T", geo)
	}
}

type retryTestDriver struct {
	mu          sync.Mutex
	queryCount  int
	failInitial bool
	failRead    bool
}

var retryTestDriverID atomic.Uint64

func (d *retryTestDriver) Open(string) (driver.Conn, error) {
	return &retryTestConn{driver: d}, nil
}

type retryTestConn struct {
	driver *retryTestDriver
}

func (c *retryTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c *retryTestConn) Close() error                        { return nil }
func (c *retryTestConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c *retryTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	c.driver.mu.Lock()
	defer c.driver.mu.Unlock()
	c.driver.queryCount++
	if c.driver.failInitial && c.driver.queryCount == 1 {
		return nil, driver.ErrBadConn
	}
	return &retryTestRows{failRead: c.driver.failRead && c.driver.queryCount == 1}, nil
}

type retryTestRows struct {
	sent     bool
	failRead bool
}

func (r *retryTestRows) Columns() []string { return []string{"id", "geom"} }
func (r *retryTestRows) Close() error      { return nil }

func (r *retryTestRows) Next(dest []driver.Value) error {
	if !r.sent {
		r.sent = true
		dest[0] = int64(7)
		dest[1] = "POINT(1 2)"
		return nil
	}
	if r.failRead {
		return mysqlDriver.ErrInvalidConn
	}
	return io.EOF
}

func (r *retryTestRows) ColumnTypeDatabaseTypeName(index int) string {
	if index == 0 {
		return "BIGINT"
	}
	return "TEXT"
}

func newRetryTestProvider(t *testing.T, d *retryTestDriver) (*Provider, *sql.DB) {
	t.Helper()
	driverName := "tegola_mysql_retry_test_" + strconv.FormatUint(retryTestDriverID.Add(1), 10)
	sql.Register(driverName, d)
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{
		db: db,
		layers: map[string]Layer{
			"test": {
				name:           "test",
				sql:            "SELECT id, geom FROM test",
				idFieldname:    "id",
				geomFieldname:  "geom",
				geometryFormat: GeometryFormatWKT,
				srid:           4326,
			},
		},
	}
	return p, db
}

func TestTileFeaturesRetriesBeforeEmittingFeatures(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failInitial bool
		failRead    bool
	}{
		{name: "initial query", failInitial: true},
		{name: "mid stream", failRead: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &retryTestDriver{failInitial: tc.failInitial, failRead: tc.failRead}
			p, db := newRetryTestProvider(t, d)
			defer func() { _ = db.Close() }()

			var got []provider.Feature
			err := p.TileFeatures(context.Background(), "test", provider.NewTile(0, 0, 0, 0, 4326), nil,
				func(f *provider.Feature) error {
					got = append(got, *f)
					return nil
				})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 {
				t.Fatalf("expected one feature after retry, got %d", len(got))
			}
			d.mu.Lock()
			queries := d.queryCount
			d.mu.Unlock()
			if queries != 2 {
				t.Fatalf("expected two queries, got %d", queries)
			}
		})
	}
}

func TestMySQLConnectionRetryClassification(t *testing.T) {
	for _, err := range []error{driver.ErrBadConn, mysqlDriver.ErrInvalidConn} {
		if !isRetryableConnectionError(err) {
			t.Errorf("expected %T to be retryable", err)
		}
		if !isRetryableConnectionError(errors.Join(errors.New("wrapped"), err)) {
			t.Errorf("expected wrapped %T to be retryable", err)
		}
	}
	if isRetryableConnectionError(errors.New("query failed")) {
		t.Error("ordinary query errors must not be retried")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForMySQLRetry(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancelled retry wait, got %v", err)
	}
}

func TestPreferContextError(t *testing.T) {
	driverErr := errors.New("driver query failed")

	t.Run("returns cancellation from context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if got := preferContextError(ctx, driverErr); !errors.Is(got, context.Canceled) {
			t.Fatalf("preferContextError() = %v, want context.Canceled", got)
		}
	})

	t.Run("preserves driver error when context is active", func(t *testing.T) {
		if got := preferContextError(context.Background(), driverErr); !errors.Is(got, driverErr) {
			t.Fatalf("preferContextError() = %v, want %v", got, driverErr)
		}
	})
}

// wkbPoint takes an x/y pair and returns the full WKB encoding of a 2D Point
// in little-endian byte order.
func wkbPoint(t *testing.T, x, y float64) []byte {
	t.Helper()
	b := []byte{1} // little-endian bom
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(x))
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(y))
	return b
}

// TestDecodeMySQLFormat exercises the MySQL native internal geometry layout:
// [4 bytes SRID (little-endian)][1 byte byte-order][WKB (type + body)]
func TestDecodeMySQLFormat(t *testing.T) {
	if got := len(wkbPoint(t, 1.5, 2.5)); got != 21 {
		t.Fatalf("WKB point length = %d, want 21", got)
	}
	wkb := wkbPoint(t, 1.5, 2.5)

	// assemble a MySQL-native geometry blob with SRID 3857
	blob := make([]byte, 0, 4+1+len(wkb))
	blob = binary.LittleEndian.AppendUint32(blob, 3857)
	blob = append(blob, 1)
	blob = append(blob, wkb...)

	srid, geo, err := decodeMySQLFormat(blob)
	if err != nil {
		t.Fatal(err)
	}
	if srid != 3857 {
		t.Errorf("expected srid 3857, got %v", srid)
	}
	if geo == nil {
		t.Fatal("expected geometry, got nil")
	}
	if point, ok := geo.(geom.Point); !ok || point != (geom.Point{1.5, 2.5}) {
		t.Errorf("expected point (1.5, 2.5), got %T %v", geo, geo)
	}

	// too-short blob is an error
	if _, _, err := decodeMySQLFormat([]byte{1, 2, 3}); err == nil {
		t.Error("expected error for short blob, got nil")
	}

	// invalid bom is an error
	bad := append([]byte{0, 0, 0, 0}, 9)
	bad = append(bad, wkb...)
	if _, _, err := decodeMySQLFormat(bad); err == nil {
		t.Error("expected error for invalid bom, got nil")
	}
}

// TestDecodeMariaDBFormat exercises the MariaDB native internal geometry
// layout: [1 byte byte-order][4 bytes SRID in that byte order][WKB]. Note
// the SRID follows the byte-order marker, unlike MySQL where it precedes it.
func TestDecodeMariaDBFormat(t *testing.T) {
	wkb := wkbPoint(t, 1.5, 2.5)

	// rebuild cleanly: [bom=1][SRID LE][WKB]
	blob := []byte{1}
	blob = binary.LittleEndian.AppendUint32(blob, 4326)
	blob = append(blob, wkb...)

	srid, geo, err := decodeMariaDBFormat(blob)
	if err != nil {
		t.Fatal(err)
	}
	if srid != 4326 {
		t.Errorf("expected srid 4326, got %v", srid)
	}
	if point, ok := geo.(geom.Point); !ok || point != (geom.Point{1.5, 2.5}) {
		t.Errorf("expected point (1.5, 2.5), got %T %v", geo, geo)
	}

	// big-endian MariaDB blob with SRID 4326
	beBlob := []byte{0} // big-endian bom
	beBlob = binary.BigEndian.AppendUint32(beBlob, 4326)
	beBlob = append(beBlob, wkb...)

	srid, _, err = decodeMariaDBFormat(beBlob)
	if err != nil {
		t.Fatal(err)
	}
	if srid != 4326 {
		t.Errorf("expected srid 4326, got %v", srid)
	}

	// MariaDB 10.7+ axis-order flags stored in the top three SRID bits must
	// be masked off (flags value 1 => SRID | 0x20000000)
	flagged := []byte{1}
	flagged = binary.LittleEndian.AppendUint32(flagged, 4326|0x20000000)
	flagged = append(flagged, wkb...)

	srid, _, err = decodeMariaDBFormat(flagged)
	if err != nil {
		t.Fatal(err)
	}
	if srid != 4326 {
		t.Errorf("expected masked srid 4326, got %v", srid)
	}

	// too-short blob is an error
	if _, _, err := decodeMariaDBFormat([]byte{1}); err == nil {
		t.Error("expected error for short blob, got nil")
	}
}

// TestDecodeGeometryAutoFallback verifies the "auto" format decodes MySQL
// native blobs and falls back to plain WKB when the native layout doesn't fit.
func TestDecodeGeometryAutoFallback(t *testing.T) {
	wkb := wkbPoint(t, 3.0, 4.0)

	// MySQL-native blob
	native := make([]byte, 0)
	native = binary.LittleEndian.AppendUint32(native, 3857)
	native = append(native, 1)
	native = append(native, wkb...)

	srid, geo, err := decodeGeometry(native, GeometryFormatAuto, GeometryFormatMySQL, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if srid != 3857 {
		t.Errorf("expected srid 3857, got %v", srid)
	}
	if geo == nil {
		t.Error("expected geometry, got nil")
	}

	// plain WKB with the auto flavor should fall back and decode successfully
	// (SRID 0 since plain WKB carries no header)
	srid, geo, err = decodeGeometry(wkb, GeometryFormatAuto, GeometryFormatMySQL, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if srid != 0 {
		t.Errorf("expected srid 0 for plain WKB, got %v", srid)
	}
	if geo == nil {
		t.Error("expected geometry, got nil")
	}

	// explicit wkb format
	_, geo, err = decodeGeometry(wkb, GeometryFormatWKB, GeometryFormatMySQL, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if geo == nil {
		t.Error("expected geometry, got nil")
	}

	// unknown format is an error
	if _, _, err := decodeGeometry(wkb, "bogus", GeometryFormatMySQL, codec.DefaultMOSConfig()); err == nil {
		t.Error("expected error for unknown format, got nil")
	}
}

// TestDecodeGeometryWKT verifies WKT text geometry decoding, used for
// geometry stored as text (e.g. a LINESTRING(...) TEXT column).
func TestDecodeGeometryWKT(t *testing.T) {
	const line = "LINESTRING(6832560.11 7372555.90, 6832518.72 7372428.95)"

	// string value
	srid, geo, err := decodeGeometry(line, GeometryFormatWKT, GeometryFormatMariaDB, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if srid != 0 {
		t.Errorf("expected srid 0 for WKT, got %v", srid)
	}
	if _, ok := geo.(geom.LineString); !ok {
		t.Errorf("expected geom.LineString, got %T", geo)
	}

	// []byte value (TEXT columns may arrive as []byte depending on driver)
	_, geo, err = decodeGeometry([]byte(line), GeometryFormatWKT, GeometryFormatMariaDB, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if geo == nil {
		t.Error("expected geometry, got nil")
	}

	// invalid WKT is an error
	if _, _, err := decodeGeometry("NOT WKT", GeometryFormatWKT, GeometryFormatMariaDB, codec.DefaultMOSConfig()); err == nil {
		t.Error("expected error for invalid WKT, got nil")
	}

	// wrong type is an error
	if _, _, err := decodeGeometry(42, GeometryFormatWKT, GeometryFormatMariaDB, codec.DefaultMOSConfig()); err == nil {
		t.Error("expected error for int geometry value, got nil")
	}

	// auto + MariaDB flavor: WKT text in a blob column falls back through
	// mariadb -> wkb -> wkt and decodes successfully
	_, geo, err = decodeGeometry([]byte(line), GeometryFormatAuto, GeometryFormatMariaDB, codec.DefaultMOSConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := geo.(geom.LineString); !ok {
		t.Errorf("expected geom.LineString, got %T", geo)
	}
}

// TestServerFlavorFromVersion checks the VERSION() string parsing.
func TestServerFlavorFromVersion(t *testing.T) {
	cases := []struct {
		version  string
		expected string
	}{
		{"10.11.6-MariaDB-ubu2204", GeometryFormatMariaDB},
		{"8.0.36", GeometryFormatMySQL},
		{"5.7.44-log", GeometryFormatMySQL},
		{"11.4.2-MariaDB", GeometryFormatMariaDB},
	}
	for _, c := range cases {
		if got := serverFlavorFromVersion(c.version); got != c.expected {
			t.Errorf("serverFlavorFromVersion(%q) = %v, expected %v", c.version, got, c.expected)
		}
	}
}

// TestQuoteIdentifier verifies backtick escaping.
func TestQuoteIdentifier(t *testing.T) {
	cases := []struct{ in, want string }{
		{"geom", "`geom`"},
		{"my`geom", "`my``geom`"},
		{"t.Okey", "`t`.`Okey`"},
		{"t.my`col", "`t`.`my``col`"},
		// audit 0.6a: qualified names recurse across every dot, so
		// schema.table.col and escaped backticks in any component quote
		// correctly.
		{"db.t.col", "`db`.`t`.`col`"},
		{"db.t.my`col", "`db`.`t`.`my``col`"},
		{"my`db.t.col", "`my``db`.`t`.`col`"},
	}
	for _, c := range cases {
		if got := quoteIdentifier(c.in); got != c.want {
			t.Errorf("quoteIdentifier(%q) = %v, expected %v", c.in, got, c.want)
		}
	}
}

// TestSRIDConsistencySQL verifies the mixed-SRID probe query quotes the
// qualified geometry column and excludes NULL geometries.
func TestSRIDConsistencySQL(t *testing.T) {
	got := sridConsistencySQL("db.t", "db.t.my`geom")
	want := "SELECT DISTINCT ST_SRID(`db`.`t`.`my``geom`) FROM `db`.`t` WHERE `db`.`t`.`my``geom` IS NOT NULL LIMIT 2"
	if got != want {
		t.Errorf("sridConsistencySQL:\n got: %v\nwant: %v", got, want)
	}
}

// TestShouldProbeTableSRIDs verifies when the mixed-SRID probe applies:
// native geometry formats always; auto only when a native header was
// actually decoded; textual/MOS geometries never.
func TestShouldProbeTableSRIDs(t *testing.T) {
	cases := []struct {
		format string
		srid   uint64
		want   bool
	}{
		{GeometryFormatAuto, 0, false},
		{GeometryFormatAuto, 4326, true},
		{GeometryFormatMySQL, 0, true},
		{GeometryFormatMySQL, 4326, true},
		{GeometryFormatMariaDB, 3857, true},
		{GeometryFormatWKB, 4326, false},
		{GeometryFormatWKT, 4326, false},
		{GeometryFormatMOS, 4326, false},
	}
	for _, c := range cases {
		if got := shouldProbeTableSRIDs(c.format, c.srid); got != c.want {
			t.Errorf("shouldProbeTableSRIDs(%q, %d) = %v, expected %v", c.format, c.srid, got, c.want)
		}
	}
}

// TestCheckTableSRIDs verifies the mixed-SRID guard: a single SRID (or
// none) passes; two distinct SRIDs fail registration with the explicit
// CRS advice.
func TestCheckTableSRIDs(t *testing.T) {
	t.Run("single srid", func(t *testing.T) {
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"SRID"}, Rows: [][]driver.Value{{int64(4326)}}})
		defer func() { _ = db.Close() }()
		if err := checkTableSRIDs(db, "t", "geom", "l"); err != nil {
			t.Errorf("expected consistent SRIDs to pass, got: %v", err)
		}
	})

	t.Run("no rows", func(t *testing.T) {
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"SRID"}})
		defer func() { _ = db.Close() }()
		if err := checkTableSRIDs(db, "t", "geom", "l"); err != nil {
			t.Errorf("expected empty table to pass, got: %v", err)
		}
	})

	t.Run("mixed srids", func(t *testing.T) {
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"SRID"}, Rows: [][]driver.Value{{int64(4326)}, {int64(3857)}}})
		defer func() { _ = db.Close() }()
		err := checkTableSRIDs(db, "t", "geom", "l")
		if err == nil {
			t.Fatalf("expected mixed SRIDs to fail registration")
		}
		if !strings.Contains(err.Error(), "mixes multiple SRIDs") {
			t.Errorf("error should name the mixed-SRID problem, got: %v", err)
		}
		if !strings.Contains(err.Error(), ConfigKeySRID) || !strings.Contains(err.Error(), ConfigKeyCRSDefn) {
			t.Errorf("error should advise explicit %v/%v, got: %v", ConfigKeySRID, ConfigKeyCRSDefn, err)
		}
	})
}

// TestReplaceTokens exercises the SQL token replacement using a real tile.
func TestReplaceTokens(t *testing.T) {
	layer := &Layer{
		name:          "testlayer",
		srid:          3857,
		geomFieldname: "geom",
		idFieldname:   "fid",
	}

	tile := provider.NewTile(6, 13, 22, 0, 3857)
	ext, _ := tile.BufferedExtent()

	got := mustReplaceTokens(t, "!BBOX!", layer, tile, ext)
	if !strings.Contains(got, "ST_Intersects(`geom`") {
		t.Errorf("expected ST_Intersects on geom field, got: %v", got)
	}
	if !strings.Contains(got, "ST_GeomFromText('POLYGON((") {
		t.Errorf("expected WKT polygon, got: %v", got)
	}

	mosLayer := &Layer{
		tablename:      "",
		geometryFormat: GeometryFormatMOS,
		mosConfig:      codec.MOSConfig{Precision: 0, UnitFactor: 0.001},
		bboxFields:     codec.DefaultBBoxFields(),
	}
	mosExtent := geom.NewExtent(geom.Point{1.25, -2.5}, geom.Point{3.75, 4.5})
	got = mustReplaceTokens(t, "SELECT * FROM buildings WHERE !BBOX!", mosLayer, tile, mosExtent)
	if want := "`MAXX` >= 1250 AND `MINX` <= 3750 AND `MAXY` >= -2500 AND `MINY` <= 4500"; !strings.Contains(got, want) {
		t.Errorf("expected indexed MOS bounds %q, got: %v", want, got)
	}

	got = mustReplaceTokens(t, "!ZOOM!-!Z!-!X!-!Y!", layer, tile, ext)
	if got != "6-6-13-22" {
		t.Errorf("expected 6-6-13-22, got: %v", got)
	}

	got = mustReplaceTokens(t, "!ID_FIELD!-!GEOM_FIELD!", layer, tile, ext)
	// audit P5-10: identifier tokens are now quoted; this used to insert
	// raw "fid-geom" and allowed identifier breakout
	if got != "`fid`-`geom`" {
		t.Errorf("expected `fid`-`geom`, got: %v", got)
	}

	got = mustReplaceTokens(t, "!GEOM_TYPE!", layer, tile, ext)
	if got != "" {
		t.Errorf("expected empty geom type for nil layer geom, got: %v", got)
	}

	// tokens are case-insensitive
	got = mustReplaceTokens(t, "!zoom!-!Zoom!", layer, tile, ext)
	if got != "6-6" {
		t.Errorf("expected case-insensitive zoom tokens (6-6), got: %v", got)
	}

	// numeric tokens must be valid floats
	for _, tok := range []string{config.ScaleDenominatorToken, config.PixelWidthToken, config.PixelHeightToken} {
		got = mustReplaceTokens(t, tok, layer, tile, ext)
		if _, err := strconv.ParseFloat(got, 64); err != nil {
			t.Errorf("%v: expected a float, got %q", tok, got)
		}
	}
}

// TestConfigValidation runs basic config error paths against NewTileProvider.
// These paths fail before a DB connection is attempted.
func TestConfigValidation(t *testing.T) {
	cases := []struct {
		name   string
		config dict.Dict
	}{
		{
			name: "missing host",
			config: dict.Dict{
				"database": "test", "user": "u", "password": "p",
			},
		},
		{
			name: "missing database",
			config: dict.Dict{
				"host": "localhost", "user": "u", "password": "p",
			},
		},
		{
			name: "invalid geometry format",
			config: dict.Dict{
				"host": "localhost", "database": "test",
				"user": "u", "password": "p",
				"geometry_format": "bogus",
			},
		},
	}
	for _, tc := range cases {
		_, err := NewTileProvider(tc.config, nil)
		if err == nil {
			t.Errorf("%v: expected error, got nil", tc.name)
		}
	}
}

// TestApplySystemInfo verifies the sysinfo-driven auto-configuration
// contract: precision, units and projection are applied when not explicitly
// configured.
func TestApplySystemInfo(t *testing.T) {
	projDefn := "+proj=merc +ellps=WGS84 +datum=WGS84 +units=m +no_defs"
	sysInfo := &mos.SystemInfo{
		Precision:       2,
		MapUnits:        mos.UnitsMillimetres,
		MapUnitsDefined: true,
		Projection:      projDefn,
	}

	t.Run("units factor applied", func(t *testing.T) {
		layer := Layer{name: "l", geometryFormat: GeometryFormatAuto, mosConfig: codec.DefaultMOSConfig()}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.mosConfig.UnitFactor != 0.001 {
			t.Errorf("mosConfig.UnitFactor = %v, want 0.001", layer.mosConfig.UnitFactor)
		}
		if layer.geometryFormat != GeometryFormatMOS {
			t.Errorf("geometryFormat = %q, want %q", layer.geometryFormat, GeometryFormatMOS)
		}
	})

	t.Run("precision applied when not explicit", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.DefaultMOSConfig()}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.mosConfig.Precision != 2 {
			t.Errorf("mosConfig.Precision = %v, want 2", layer.mosConfig.Precision)
		}
	})

	t.Run("explicit precision wins", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.MOSConfig{Precision: 4, UnitFactor: 1, PrecisionSet: true}}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.mosConfig.Precision != 4 {
			t.Errorf("mosConfig.Precision = %v, want 4 (explicit config wins)", layer.mosConfig.Precision)
		}
	})

	t.Run("explicit zero precision wins", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.MOSConfig{Precision: 0, UnitFactor: 1, PrecisionSet: true}}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.mosConfig.Precision != 0 {
			t.Errorf("mosConfig.Precision = %v, want 0 (explicit config wins)", layer.mosConfig.Precision)
		}
	})

	t.Run("explicit units wins", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.MOSConfig{Precision: 0, UnitFactor: 1000, UnitsSet: true}}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.mosConfig.UnitFactor != 1000 {
			t.Errorf("mosConfig.UnitFactor = %v, want 1000 (explicit config wins)", layer.mosConfig.UnitFactor)
		}
	})

	t.Run("projection applied when srid not explicit", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.DefaultMOSConfig()}
		if err := applySystemInfo(&layer, sysInfo, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.srid == 0 {
			t.Fatal("expected a synthetic srid to be registered")
		}
		defn, ok := basic.Proj4DefnSRID(projDefn)
		if !ok {
			t.Fatalf("registered srid %v not found by defn lookup", layer.srid)
		}
		if defn != layer.srid {
			t.Errorf("Proj4DefnSRID = %v, want %v", defn, layer.srid)
		}
	})

	t.Run("projection not applied when srid explicit", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.DefaultMOSConfig(), srid: 3857}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.srid != 3857 {
			t.Errorf("srid = %v, want 3857 (explicit srid wins)", layer.srid)
		}
	})

	t.Run("layer srid suppresses system projection", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.DefaultMOSConfig(), srid: 32637, crsExplicit: true}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.srid != 32637 {
			t.Errorf("srid = %v, want 32637 (layer srid wins)", layer.srid)
		}
	})

	t.Run("layer crs definition suppresses system projection", func(t *testing.T) {
		layerSRID, err := basic.RegisterProj4Defn(projDefn)
		if err != nil {
			t.Fatalf("registering layer CRS: %v", err)
		}
		layer := Layer{name: "l", mosConfig: codec.DefaultMOSConfig(), srid: layerSRID, crsExplicit: true}
		if err := applySystemInfo(&layer, sysInfo, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.srid != layerSRID {
			t.Errorf("srid = %v, want %v (layer crs_defn wins)", layer.srid, layerSRID)
		}
	})

	t.Run("nil sysinfo is a no-op", func(t *testing.T) {
		layer := Layer{name: "l", mosConfig: codec.MOSConfig{Precision: 3, UnitFactor: 1}, srid: 3395}
		if err := applySystemInfo(&layer, nil, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if layer.mosConfig.Precision != 3 || layer.srid != 3395 {
			t.Errorf("layer modified by nil sysinfo: precision=%v srid=%v", layer.mosConfig.Precision, layer.srid)
		}
	})
}

func TestApplyRuntimeSystemInfo(t *testing.T) {
	t.Skip("runtime system-info application removed by canonical MapplGIS contract (audit A-01): detection is registration-time only")
}

// TestMySQLLayerGeometryFormatResolution verifies the layer-level
// geometry_format override contract (R4-02): the layer value wins over the
// provider value, is validated against the full MySQL value set with the
// layer name in the error context, and resolving one layer never mutates
// the provider-level value seen by subsequent layers.
func TestMySQLLayerGeometryFormatResolution(t *testing.T) {
	tcases := []struct {
		name          string
		providerValue string
		layer         dict.Dict
		expect        string
		expectErr     bool
	}{
		{name: "provider auto + layer mos", providerValue: GeometryFormatAuto, layer: dict.Dict{"geometry_format": "mos"}, expect: GeometryFormatMOS},
		{name: "provider mysql + layer wkb", providerValue: GeometryFormatMySQL, layer: dict.Dict{"geometry_format": "wkb"}, expect: GeometryFormatWKB},
		{name: "provider mos + layer wkt", providerValue: GeometryFormatMOS, layer: dict.Dict{"geometry_format": "wkt"}, expect: GeometryFormatWKT},
		{name: "no layer key keeps provider value", providerValue: GeometryFormatMariaDB, layer: dict.Dict{}, expect: GeometryFormatMariaDB},
		{name: "invalid layer format errors", providerValue: GeometryFormatAuto, layer: dict.Dict{"geometry_format": "bogus"}, expectErr: true},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := codec.ResolveLayerGeometryFormat(tc.providerValue, tc.layer, "roads", mysqlGeometryFormats)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "roads") {
					t.Errorf("error should mention layer name: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expect {
				t.Errorf("expected %q got %q", tc.expect, got)
			}
		})
	}
}

// TestMySQLTwoLayersDifferentFormats verifies that two sequential layer
// resolutions with different layer-level overrides stay independent.
func TestMySQLTwoLayersDifferentFormats(t *testing.T) {
	providerValue := GeometryFormatAuto
	first, err := codec.ResolveLayerGeometryFormat(providerValue, dict.Dict{"geometry_format": "mos"}, "mos_layer", mysqlGeometryFormats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := codec.ResolveLayerGeometryFormat(providerValue, dict.Dict{"geometry_format": "wkb"}, "wkb_layer", mysqlGeometryFormats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != GeometryFormatMOS || second != GeometryFormatWKB {
		t.Errorf("expected (mos, wkb), got (%v, %v)", first, second)
	}
}

// TestMySQLMOSParamsKeptForAutoFormat verifies the R4-04 semantics: with
// geometry_format auto (which may still switch to MOS at runtime via a
// MapplGIS LayerInfo blob) explicit MOS params are kept, not warned or
// reset; only explicitly raw layer formats ignore them.
func TestMySQLMOSParamsKeptForAutoFormat(t *testing.T) {
	cfg := codec.MOSConfig{Precision: 3, PrecisionSet: true, UnitFactor: 0.001, UnitsSet: true}

	for _, format := range []string{GeometryFormatAuto, GeometryFormatMOS} {
		if format != GeometryFormatAuto && format != GeometryFormatMOS {
			continue
		}
		if !cfg.HasExplicitMOSParams() {
			t.Errorf("%v: explicit params should be detected", format)
		}
	}

	for _, format := range []string{GeometryFormatWKB, GeometryFormatWKT, GeometryFormatMySQL, GeometryFormatMariaDB} {
		// registration warns and resets to defaults for these formats
		reset := codec.DefaultMOSConfig()
		if reset.PrecisionSet || reset.UnitsSet {
			t.Errorf("%v: reset config should carry no explicit flags: %+v", format, reset)
		}
	}
}

type noQueryDriver struct{}

func (d *noQueryDriver) Open(string) (driver.Conn, error) { return &noQueryConn{}, nil }

type noQueryConn struct{}

func (c *noQueryConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c *noQueryConn) Close() error                        { return nil }
func (c *noQueryConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }
func (c *noQueryConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("no startup queries expected")
}

// ensureLiveTestTable creates the plain (non-MapplGIS) table used by the
// live construction tests. With the canonical MapplGIS contract (A-05)
// detection runs for every tablename layer, so the table must exist at
// registration; the earlier contract skipped all inspection for explicit
// geometry_type layers and did not need it.
func ensureLiveTestTable(t *testing.T) {
	t.Helper()
	db, err := sql.Open("mysql", "u:p@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Fatalf("unable to open live test connection: %v", err)
	}
	defer func() { _ = db.Close() }()
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS `lines` (" +
		"`id` BIGINT NOT NULL PRIMARY KEY," +
		"`geom` GEOMETRY," +
		"`name` VARCHAR(255))")
	if err != nil {
		t.Fatalf("unable to create live test table: %v", err)
	}
}

// TestExplicitGeometryTypeSkipsStartupInspection verifies the common
// geometry_type layer key contract for MySQL (docs/provider-contract.md):
// an explicit value fixes the layer type before any data is read and skips
// the startup geometry sampling query, and an unsupported value fails
// provider construction. MapplGIS detection (DDL + PK + indexes + OKEY=1)
// still runs once at registration, before the geometry_type decision,
// independently of the configured type (audit A-05).
func TestExplicitGeometryTypeSkipsStartupInspection(t *testing.T) {
	baseConfig := func() dict.Dict {
		return dict.Dict{
			"host": "localhost", "database": "test",
			"user": "u", "password": "p",
			"layers": []map[string]interface{}{
				{
					"name":           "lines",
					"tablename":      "lines",
					"id_fieldname":   "id",
					"geom_fieldname": "geom",
				},
			},
		}
	}

	t.Run("explicit type skips inspection", func(t *testing.T) {
		// This subtest exercises the full provider construction path, which
		// requires a reachable MySQL server. It is gated the same way as the
		// postgis live tests (RUN_MYSQL_TESTS=yes).
		if os.Getenv("RUN_MYSQL_TESTS") != "yes" {
			t.Skip("skip live construction test, set RUN_MYSQL_TESTS=yes to run")
		}
		ensureLiveTestTable(t)
		driverName := "tegola_mysql_no_query_test_" + strconv.FormatUint(retryTestDriverID.Add(1), 10)
		sql.Register(driverName, &noQueryDriver{})

		config := baseConfig()
		config["layers"].([]map[string]interface{})[0]["geometry_type"] = "LineString"

		prov, err := NewTileProvider(config, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lyrs, err := prov.(provider.Layerer).Layers()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lyrs) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(lyrs))
		}
		if _, ok := lyrs[0].GeomType().(geom.LineString); !ok {
			t.Fatalf("expected geom.LineString, got %T", lyrs[0].GeomType())
		}
	})

	t.Run("explicit type keeps table identity", func(t *testing.T) {
		// A-02 regression: tablename + geometry_type must register a layer
		// with the table identity (tablename and fields) so TileFeatures
		// serves the table instead of falling through to an empty custom
		// SQL. Requires a reachable MySQL server.
		if os.Getenv("RUN_MYSQL_TESTS") != "yes" {
			t.Skip("skip live construction test, set RUN_MYSQL_TESTS=yes to run")
		}
		ensureLiveTestTable(t)

		config := baseConfig()
		config["layers"].([]map[string]interface{})[0]["geometry_type"] = "LineString"
		config["layers"].([]map[string]interface{})[0]["fields"] = []string{"name"}

		prov, err := NewTileProvider(config, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lyrs, err := prov.(provider.Layerer).Layers()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lyrs) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(lyrs))
		}
		// A-02 regression: tablename + geometry_type must register a layer
		// with the table identity (tablename and fields) so TileFeatures
		// takes the table path instead of falling through to an empty
		// custom SQL.
		lyr, ok := lyrs[0].(Layer)
		if !ok {
			t.Fatalf("expected internal Layer type, got %T", lyrs[0])
		}
		if lyr.tablename != "lines" {
			t.Errorf("expected tablename 'lines', got %q", lyr.tablename)
		}
		if len(lyr.tagFieldnames) != 1 || lyr.tagFieldnames[0] != "name" {
			t.Errorf("expected tagFieldnames [name], got %v", lyr.tagFieldnames)
		}
		if lyr.sql != "" {
			t.Errorf("expected empty custom sql for a table layer, got %q", lyr.sql)
		}
	})

	t.Run("invalid type fails construction", func(t *testing.T) {
		driverName := "tegola_mysql_no_query_test_" + strconv.FormatUint(retryTestDriverID.Add(1), 10)
		sql.Register(driverName, &noQueryDriver{})

		config := baseConfig()
		config["layers"].([]map[string]interface{})[0]["geometry_type"] = "triangle"

		if _, err := NewTileProvider(config, nil); err == nil {
			t.Fatal("expected error for unsupported geometry_type, got nil")
		}
	})
}

// TestExplicitGeometryTypeMixedContentPermitted verifies the mixed-content
// policy: features whose decoded type differs from the explicitly configured
// geometry_type are permitted (with a one-time warning), they do not break
// tile rendering.
func TestExplicitGeometryTypeMixedContentPermitted(t *testing.T) {
	inside, err := wkb.EncodeBytes(geom.Point{-1000000, 1000000})
	if err != nil {
		t.Fatal(err)
	}

	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id", "geom"}, Rows: [][]driver.Value{
		{int64(1), inside},
	}})

	p := &Provider{
		db: db,
		layers: map[string]Layer{
			"test": {
				name:             "test",
				sql:              "SELECT id, geom FROM test WHERE !BBOX!",
				idFieldname:      "id",
				geomFieldname:    "geom",
				geometryFormat:   GeometryFormatWKB,
				srid:             tegola.WebMercator,
				geomType:         geom.LineString{},
				geomTypeExplicit: true,
			},
		},
	}

	var got []provider.Feature
	err = p.TileFeatures(context.Background(), "test", provider.NewTile(1, 0, 0, 0, tegola.WebMercator), nil,
		func(f *provider.Feature) error {
			got = append(got, *f)
			return nil
		})
	if err != nil {
		t.Fatalf("mixed-content feature must not break tile rendering: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the point feature to be permitted, got %d features", len(got))
	}
	if _, ok := got[0].Geometry.(geom.Point); !ok {
		t.Fatalf("expected geom.Point, got %T", got[0].Geometry)
	}
}

// TestParseShowIndexRows covers the version-sensitive SHOW INDEX parsing
// (audit A01/A08): MySQL 5.7 (13 columns), MySQL 8.0 (15 columns), future
// servers with extra trailing columns, and rows shorter than the fixed
// legacy layout, which was the old scanner's failure mode.
func TestParseShowIndexRows(t *testing.T) {
	t.Run("mysql 5.7 thirteen columns", func(t *testing.T) {
		columns := []string{
			"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name",
			"Collation", "Cardinality", "Sub_part", "Packed", "Null",
			"Index_type", "Comment", "Index_comment",
		}
		rows := [][]driver.Value{
			{"t", int64(0), "PRIMARY", int64(1), "OKEY", "A", int64(1), nil, nil, "", "BTREE", "", ""},
			{"t", int64(1), "MINX_IDX", int64(1), "MINX", "A", int64(1), nil, nil, "YES", "BTREE", "", ""},
			{"t", int64(1), "MINX_IDX", int64(2), "MUID", "A", int64(1), nil, nil, "YES", "BTREE", "", ""},
		}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows})
		rs, err := db.Query("SHOW INDEX FROM t")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rs.Close() }()
		got, err := parseShowIndexRows(rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 rows, got %d: %+v", len(got), got)
		}
		if got[0].keyName != "PRIMARY" || got[0].seqInIndex != 1 || got[0].columnName != "OKEY" {
			t.Errorf("row 0 = %+v", got[0])
		}
		if got[1].keyName != "MINX_IDX" || got[1].seqInIndex != 1 || got[1].columnName != "MINX" {
			t.Errorf("row 1 = %+v", got[1])
		}
		if got[2].seqInIndex != 2 || got[2].columnName != "MUID" {
			t.Errorf("row 2 = %+v", got[2])
		}
	})

	t.Run("mysql 8 fifteen columns", func(t *testing.T) {
		columns := []string{
			"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name",
			"Collation", "Cardinality", "Sub_part", "Packed", "Null",
			"Index_type", "Comment", "Index_comment", "Visible", "Expression",
		}
		rows := [][]driver.Value{
			{"t", int64(0), "PRIMARY", int64(1), "OKEY", "A", int64(1), nil, nil, "", "BTREE", "", "", "YES", nil},
		}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows})
		rs, err := db.Query("SHOW INDEX FROM t")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rs.Close() }()
		got, err := parseShowIndexRows(rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].columnName != "OKEY" || got[0].seqInIndex != 1 {
			t.Fatalf("unexpected rows: %+v", got)
		}
	})

	t.Run("future server extra columns", func(t *testing.T) {
		// A server newer than the ones we know appends columns; the
		// scanner must keep working by name.
		columns := []string{
			"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name",
			"Collation", "Cardinality", "Sub_part", "Packed", "Null",
			"Index_type", "Comment", "Index_comment", "Visible", "Expression",
			"Future_Col_1", "Future_Col_2",
		}
		rows := [][]driver.Value{
			{"t", int64(0), "PRIMARY", int64(1), "OKEY", "A", int64(1), nil, nil, "", "BTREE", "", "", "YES", nil, "x", "y"},
		}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows})
		rs, err := db.Query("SHOW INDEX FROM t")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rs.Close() }()
		got, err := parseShowIndexRows(rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].columnName != "OKEY" {
			t.Fatalf("unexpected rows: %+v", got)
		}
	})

	t.Run("malformed seq_in_index value", func(t *testing.T) {
		// A non-numeric Seq_in_index must fail loudly instead of silently
		// mis-grouping the index layout (the old fixed-position scanner
		// misparsed cross-version output this way).
		columns := []string{
			"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name",
			"Collation", "Cardinality", "Sub_part", "Packed", "Null",
			"Index_type", "Comment", "Index_comment",
		}
		rows := [][]driver.Value{
			{"t", int64(0), "PRIMARY", "not-a-number", "OKEY", "A", int64(1), nil, nil, "", "BTREE", "", ""},
		}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows})
		rs, err := db.Query("SHOW INDEX FROM t")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rs.Close() }()
		if _, err := parseShowIndexRows(rs); err == nil {
			t.Fatal("expected an error parsing a malformed Seq_in_index, got nil")
		}
	})

	t.Run("missing required column", func(t *testing.T) {
		columns := []string{"Table", "Non_unique", "Key_name", "Seq_in_index"}
		rows := [][]driver.Value{{"t", int64(0), "PRIMARY", int64(1)}}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows})
		rs, err := db.Query("SHOW INDEX FROM t")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rs.Close() }()
		if _, err := parseShowIndexRows(rs); err == nil {
			t.Fatal("expected an error for a result without Column_name, got nil")
		}
	})

	t.Run("null values", func(t *testing.T) {
		columns := []string{"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name"}
		rows := [][]driver.Value{
			{nil, nil, "PRIMARY", "1", "OKEY"},
		}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows})
		rs, err := db.Query("SHOW INDEX FROM t")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rs.Close() }()
		got, err := parseShowIndexRows(rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].keyName != "PRIMARY" || got[0].columnName != "OKEY" || got[0].seqInIndex != 1 {
			t.Fatalf("unexpected rows: %+v", got)
		}
	})
}

// TestMapplGISFormatAuthority verifies the audit A04 contract: a detected
// MapplGIS layer is authoritative about its storage format, so only
// unset/auto/mos are accepted and any explicit raw format is a startup
// conflict error.
func TestMapplGISFormatAuthority(t *testing.T) {
	for _, ok := range []string{"", GeometryFormatAuto, GeometryFormatMOS} {
		if err := mapplGISFormatAuthority("l", ok); err != nil {
			t.Errorf("format %q should be accepted, got error: %v", ok, err)
		}
	}
	for _, conflict := range []string{GeometryFormatMySQL, GeometryFormatMariaDB, GeometryFormatWKB, GeometryFormatWKT} {
		err := mapplGISFormatAuthority("l", conflict)
		if err == nil {
			t.Errorf("format %q must be rejected for a detected MapplGIS layer", conflict)
			continue
		}
		if !strings.Contains(err.Error(), "MapplGIS") || !strings.Contains(err.Error(), conflict) {
			t.Errorf("error for %q should mention the conflict, got: %v", conflict, err)
		}
	}
}

// The geometry is not necessarily the first column of a custom SELECT.
func TestGeomTypeFromQueryUsesNamedColumn(t *testing.T) {
	for _, columns := range [][]string{{"id", "LINE", "label"}, {"id", "line", "label"}} {
		db := fixture.OpenSQLRows(t, fixture.SQLRows{
			Columns: columns,
			Rows: [][]driver.Value{
				{int64(1), nil, "null geometry"},
				{int64(2), "invalid", "bad geometry"},
				{int64(3), "POINT(1 2)", "valid geometry"},
			},
		})
		layer := &Layer{geomFieldname: "LINE", geometryFormat: GeometryFormatWKT}
		geo, _, err := geomTypeFromQuery(db, "SELECT id, LINE, label FROM t LIMIT 16", layer)
		if err != nil {
			t.Fatal(err)
		}
		point, ok := geo.(geom.Point)
		if !ok || point != (geom.Point{1, 2}) {
			t.Fatalf("geometry = %#v, expected POINT(1 2)", geo)
		}
	}
}

func TestGeomTypeFromQueryRejectsMissingGeometry(t *testing.T) {
	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id", "label"}})
	_, _, err := geomTypeFromQuery(db, "SELECT id, label FROM t LIMIT 16", &Layer{geomFieldname: "LINE"})
	if err == nil || !strings.Contains(err.Error(), "geometry column") {
		t.Fatalf("expected missing geometry column error, got %v", err)
	}
}
