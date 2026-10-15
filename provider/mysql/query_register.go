package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

type featureColumn struct {
	name, dataType, columnType, nullable, extra, collation string
	characterSet                                           string
	srid                                                   sql.NullInt64
}

type featureIndexColumn struct {
	name, column        string
	position, nonUnique int
	prefix              sql.NullInt64
}

type featureSchema struct {
	lowerCaseTables                    int
	database, table, engine, tableType string
	physicalID, serverVersion          string
	legacyIdentity                     bool
	columns                            []featureColumn
	indexes                            []featureIndexColumn
}

type featureProjection struct{ source, output string }

type featureProfile struct {
	publicTemporalNulls bool
	schema              featureSchema
	projection          []featureProjection
	properties          []string
	id, geometry        string
	idUnsigned          bool
	idBits              int
	format              string
	native              bool
	nativeAxisOption    bool
	nativeSRIDLabel     string
	srid                uint64
	spatial             provider.SpatialMetadata
	height              *crsconfig.HeightProjection
	crs                 provider.FeatureCRSDefinition
	crsProjection       *crsconfig.FeatureProjection
	crsDeclared         bool
	temporal            provider.TemporalMapping
	temporalScale       int64
	mos                 codec.MOSConfig
	bounds              [4]string
	filter              string
	filterArgs          []any
	queryableCatalog    provider.FeatureQueryables
	queryableColumns    map[string]featureFilterColumn
	queryableError      error
}

func featureInvalid(field, reason string) error {
	return provider.InvalidFeatureQueryError{Field: field, Reason: reason}
}

func featureUnsupported(reason string) error {
	return fmt.Errorf("mysql feature profile %s: %w", reason, provider.ErrUnsupported)
}

func (l Layer) FeatureQuerySupported() error {
	if l.featureError != nil {
		return l.featureError
	}
	if l.feature == nil {
		return featureUnsupported("not registered")
	}
	return nil
}

func (l Layer) FeatureSourceSRID() uint64 {
	if l.feature == nil || l.featureError != nil {
		return 0
	}
	return l.feature.srid
}

func (l Layer) TemporalMapping() (provider.TemporalMapping, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.TemporalMapping{}, err
	}
	return l.feature.temporal, nil
}

func (l Layer) SpatialMetadata() (provider.SpatialMetadata, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.SpatialMetadata{}, err
	}
	return l.feature.spatial, nil
}

func (p *Provider) registerFeatureLayers(confs []dict.Dicter) error {
	for _, conf := range confs {
		name, err := conf.String(ConfigKeyLayerName, nil)
		if err != nil {
			return err
		}
		layer := p.layers[name]
		profile, err := p.registerFeatureLayer(layer, conf)
		var invalid provider.InvalidFeatureQueryError
		if errors.As(err, &invalid) {
			return err
		}
		if err == nil {
			profile.registerFeatureBounds(layer, conf)
			if profile.schema.legacyIdentity {
				p.legacyIdentityWarning.Do(func() {
					log.Warn("mysql 5.5 legacy table identity enabled: metadata drift checks cannot prove physical incarnation; external DDL is unsupported")
				})
			}
			profile.crsDeclared = layer.crsExplicit
			profile.freezeFeatureCRS()
			profile.initializeFilterCatalog()
		}
		layer.feature, layer.featureError = profile, err
		p.layers[name] = layer
	}
	return nil
}

func (p *Provider) registerFeatureLayer(layer Layer, conf dict.Dicter) (*featureProfile, error) {
	// Explicit syntax is checked even when source capability is unsupported.
	if _, _, err := featureTemporalConfig(conf); err != nil {
		return nil, err
	}
	options := &featureProfile{srid: layer.srid, format: layer.geometryFormat}
	if err := options.registerSpatial(conf, layer); err != nil {
		var invalid provider.InvalidFeatureQueryError
		if errors.As(err, &invalid) {
			return nil, err
		}
	}
	if raw, exists := conf.Interface("feature_sql"); exists {
		text, ok := raw.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, featureInvalid("feature_sql", "must be nonblank SQL")
		}
		plan, err := featuresql.Parse(text, featuresql.MySQL)
		if err != nil {
			return nil, featureSQLAdmissionError(err)
		}
		return p.registerFeatureSelection(layer, conf, plan)
	}
	if layer.tablename == "" {
		return nil, featureUnsupported("custom tile SQL requires feature_sql")
	}
	ctx, cancel := codec.NewInspectionContext()
	defer cancel()
	schema, err := inspectFeatureSchema(ctx, p.db, p.Database, layer.tablename, p.allowLegacyTableIdentity)
	if err != nil {
		return nil, fmt.Errorf("mysql feature catalog: %w", err)
	}
	profile := &featureProfile{schema: schema, id: layer.idFieldname, geometry: layer.geomFieldname,
		format: layer.geometryFormat, srid: layer.srid, mos: layer.mosConfig, filter: "1"}
	if err := profile.resolveOrdinary(layer); err != nil {
		return nil, err
	}
	if err := profile.registerTemporal(conf); err != nil {
		return nil, err
	}
	if err := profile.registerSpatial(conf, layer); err != nil {
		return nil, err
	}
	if err := profile.registerNative(ctx, p.db, p.serverFlavor, layer.crsExplicit); err != nil {
		return nil, err
	}
	return profile, nil
}

type featureCatalogQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func inspectFeatureSchema(ctx context.Context, db featureCatalogQuerier, database, table string, allowLegacy bool) (featureSchema, error) {
	result := featureSchema{database: database, table: table}
	err := withFeatureRows(ctx, db, "SELECT @@lower_case_table_names", []any{}, func(rows *sql.Rows) error {
		if !rows.Next() {
			return featureUnsupported("table-name rules unavailable")
		}
		return rows.Scan(&result.lowerCaseTables)
	})
	if err != nil {
		return result, err
	}
	if result.lowerCaseTables < 0 || result.lowerCaseTables > 2 {
		return result, featureUnsupported("table-name rules")
	}
	relationSQL := "SELECT TABLE_SCHEMA,TABLE_NAME,TABLE_TYPE,ENGINE FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?"
	if result.lowerCaseTables == 0 {
		relationSQL = "SELECT TABLE_SCHEMA,TABLE_NAME,TABLE_TYPE,ENGINE FROM INFORMATION_SCHEMA.TABLES WHERE BINARY TABLE_SCHEMA=BINARY ? AND BINARY TABLE_NAME=BINARY ?"
	}
	err = withFeatureRows(ctx, db, relationSQL,
		[]any{database, table}, func(rows *sql.Rows) error {
			if !rows.Next() {
				return featureUnsupported("base relation not visible")
			}
			var engine sql.NullString
			if err := rows.Scan(&result.database, &result.table, &result.tableType, &engine); err != nil {
				return err
			}
			result.engine = engine.String
			if rows.Next() {
				return featureUnsupported("ambiguous relation")
			}
			return nil
		})
	if err != nil {
		return result, err
	}
	if result.tableType != "BASE TABLE" || !strings.EqualFold(result.engine, "InnoDB") {
		return result, featureUnsupported("requires InnoDB base table")
	}
	database, table = result.database, result.table
	err = withFeatureRows(ctx, db, "SELECT VERSION()", []any{}, func(rows *sql.Rows) error {
		if !rows.Next() {
			return featureUnsupported("server version unavailable")
		}
		return rows.Scan(&result.serverVersion)
	})
	if err != nil {
		return result, err
	}
	major, err := strconv.Atoi(strings.SplitN(result.serverVersion, ".", 2)[0])
	if err != nil {
		return result, featureUnsupported("server version unknown")
	}
	nativeCatalog := "INFORMATION_SCHEMA.INNODB_SYS_TABLES"
	mysql8 := !strings.Contains(strings.ToLower(result.serverVersion), "mariadb") && major == 8
	if mysql8 {
		nativeCatalog = "INFORMATION_SCHEMA.INNODB_TABLES"
	}
	// The opt-in never turns a missing/denied modern native catalog into a
	// weaker identity. MySQL 5.5 alone lacks this catalog by design.
	legacy := allowLegacy && strings.HasPrefix(result.serverVersion, "5.5.") &&
		!strings.Contains(strings.ToLower(result.serverVersion), "mariadb")
	var created string
	if legacy {
		result.legacyIdentity = true
		err = withFeatureRows(ctx, db,
			"SELECT DATE_FORMAT(CREATE_TIME,'%Y-%m-%d %H:%i:%s') FROM INFORMATION_SCHEMA.TABLES WHERE BINARY TABLE_SCHEMA=BINARY ? AND BINARY TABLE_NAME=BINARY ?",
			[]any{database, table}, func(rows *sql.Rows) error {
				if !rows.Next() {
					return featureUnsupported("legacy creation metadata unavailable")
				}
				var value sql.NullString
				if err := rows.Scan(&value); err != nil {
					return err
				}
				if !value.Valid || value.String == "" || rows.Next() {
					return featureUnsupported("legacy creation metadata unproven")
				}
				created = value.String
				return nil
			})
	} else {
		err = withFeatureRows(ctx, db, "SELECT TABLE_ID FROM "+nativeCatalog+" WHERE NAME=?", []any{database + "/" + table}, func(rows *sql.Rows) error {
			if !rows.Next() {
				return featureUnsupported("stable physical table identity unavailable")
			}
			if err := rows.Scan(&result.physicalID); err != nil {
				return err
			}
			if result.physicalID == "" || result.physicalID == "0" || rows.Next() {
				return featureUnsupported("physical table identity unproven")
			}
			return nil
		})
	}
	if err != nil {
		return result, errors.Join(featureUnsupported("stable physical table identity catalog"), err)
	}
	columnSQL := "SELECT COLUMN_NAME,DATA_TYPE,COLUMN_TYPE,IS_NULLABLE,EXTRA,COALESCE(COLLATION_NAME,''),COALESCE(CHARACTER_SET_NAME,'')"
	if mysql8 {
		columnSQL += ",SRS_ID"
	}
	columnSQL += " FROM INFORMATION_SCHEMA.COLUMNS WHERE BINARY TABLE_SCHEMA=BINARY ? AND BINARY TABLE_NAME=BINARY ? ORDER BY ORDINAL_POSITION"
	err = withFeatureRows(ctx, db,
		columnSQL,
		[]any{database, table}, func(rows *sql.Rows) error {
			for rows.Next() {
				var column featureColumn
				targets := []any{
					&column.name, &column.dataType, &column.columnType, &column.nullable,
					&column.extra, &column.collation, &column.characterSet,
				}
				if mysql8 {
					targets = append(targets, &column.srid)
				}
				if err := rows.Scan(targets...); err != nil {
					return err
				}
				result.columns = append(result.columns, column)
			}
			return nil
		})
	if err != nil {
		return result, err
	}
	err = withFeatureRows(ctx, db,
		"SELECT INDEX_NAME,SEQ_IN_INDEX,NON_UNIQUE,COALESCE(COLUMN_NAME,''),SUB_PART FROM INFORMATION_SCHEMA.STATISTICS WHERE BINARY TABLE_SCHEMA=BINARY ? AND BINARY TABLE_NAME=BINARY ? ORDER BY INDEX_NAME,SEQ_IN_INDEX",
		[]any{database, table}, func(rows *sql.Rows) error {
			for rows.Next() {
				var column featureIndexColumn
				if err := rows.Scan(&column.name, &column.position, &column.nonUnique, &column.column, &column.prefix); err != nil {
					return err
				}
				result.indexes = append(result.indexes, column)
			}
			return nil
		})
	if err != nil {
		return result, err
	}
	if legacy {
		// The hash covers the whole captured catalog, including creation time,
		// ordered columns and indexes. The read path repeats this under MDL.
		// Timestamp granularity still cannot distinguish same-shape recreation
		// within one second; this is deliberately not a native incarnation ID.
		digest := sha256.Sum256([]byte(fmt.Sprintf("%#v|%q", result, created)))
		result.physicalID = fmt.Sprintf("mysql55-metadata:%x", digest)
	}
	return result, nil
}

func withFeatureRows(ctx context.Context, db featureCatalogQuerier, statement string, args []any, read func(*sql.Rows) error) (err error) {
	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return preferContextError(ctx, err)
	}
	defer func() {
		err = errors.Join(err, rows.Err(), rows.Close())
		if ctx.Err() != nil {
			err = errors.Join(ctx.Err(), err)
		}
	}()
	return read(rows)
}

func (s featureSchema) column(name string) (featureColumn, bool) {
	// MySQL column names are case-insensitive; return the catalog spelling.
	for _, column := range s.columns {
		if strings.EqualFold(column.name, name) {
			return column, true
		}
	}
	return featureColumn{}, false
}

func integralColumn(column featureColumn) (bits int, unsigned bool) {
	switch strings.ToLower(column.dataType) {
	case "tinyint":
		bits = 8
	case "smallint":
		bits = 16
	case "mediumint":
		bits = 24
	case "int", "integer":
		bits = 32
	case "bigint":
		bits = 64
	}
	return bits, strings.Contains(strings.ToLower(column.columnType), "unsigned")
}

func generatedColumn(column featureColumn) bool {
	return strings.Contains(strings.ToLower(column.extra), "generated")
}

func (f *featureProfile) resolveOrdinary(layer Layer) error {
	id, ok := f.schema.column(f.id)
	if !ok || generatedColumn(id) {
		return featureUnsupported("identity column missing or generated")
	}
	f.id, f.idBits, f.idUnsigned = id.name, 0, false
	f.idBits, f.idUnsigned = integralColumn(id)
	if f.idBits == 0 {
		return featureUnsupported("identity is not integral")
	}
	unique := false
	for i, index := range f.schema.indexes {
		if index.nonUnique != 0 || index.position != 1 || index.column != f.id || index.prefix.Valid {
			continue
		}
		if i+1 < len(f.schema.indexes) && f.schema.indexes[i+1].name == index.name {
			continue
		}
		unique = true
	}
	if !unique {
		return featureUnsupported("single-column identity uniqueness unproven")
	}
	geometry, ok := f.schema.column(f.geometry)
	if !ok || generatedColumn(geometry) {
		return featureUnsupported("geometry column missing or generated")
	}
	f.geometry = geometry.name
	for _, column := range f.schema.columns {
		f.projection = append(f.projection, featureProjection{source: column.name, output: column.name})
	}
	for _, column := range f.schema.columns {
		if column.name == f.id || column.name == f.geometry || featurePrivateColumn(layer, column.name) {
			continue
		}
		if len(layer.tagFieldnames) != 0 && !containsFeatureFold(layer.tagFieldnames, column.name) {
			continue
		}
		f.properties = append(f.properties, column.name)
	}
	for _, requested := range layer.tagFieldnames {
		if _, ok := f.schema.column(requested); !ok {
			return featureInvalid("fields", "configured source property missing")
		}
	}
	return nil
}

func featurePrivateColumn(layer Layer, name string) bool {
	return containsFeatureFold(layer.bboxFields[:], name) || strings.EqualFold(name, "min_zoom") || strings.EqualFold(name, "max_zoom")
}

func containsFeatureFold(names []string, value string) bool {
	for _, name := range names {
		if strings.EqualFold(name, value) {
			return true
		}
	}
	return false
}

func (f *featureProfile) relation() string {
	return featureQuoteIdentifier(f.schema.database) + "." + featureQuoteIdentifier(f.schema.table)
}

func featureQuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
