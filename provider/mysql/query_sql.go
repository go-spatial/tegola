package mysql

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
	"github.com/alexeydott/tegola/provider/mapplgis"
)

func featureSQLAdmissionError(err error) error {
	if errors.Is(err, featuresql.ErrInvalid) {
		return featureInvalid("feature_sql", "invalid selection configuration")
	}
	if errors.Is(err, featuresql.ErrUnsupported) {
		return errors.Join(featureUnsupported("custom selection"), err)
	}
	return err
}

type mysqlFeatureCatalog struct {
	ctx                      context.Context
	db                       *sql.DB
	database                 string
	lowerCaseTables          int
	schema                   featureSchema
	allowLegacyTableIdentity bool
}

func (c *mysqlFeatureCatalog) ResolveRelation(name featuresql.Name) (featuresql.RelationMetadata, error) {
	parts := name.Parts()
	if len(parts) < 1 || len(parts) > 2 {
		return featuresql.RelationMetadata{}, featureUnsupported("relation arity")
	}
	database, table := c.database, parts[len(parts)-1].Name()
	if len(parts) == 2 {
		database = parts[0].Name()
	}
	if c.lowerCaseTables != 0 {
		database, table = strings.ToLower(database), strings.ToLower(table)
	}
	// Read canonical catalog names and filter using table-name rules, instead
	// of letting INFORMATION_SCHEMA's default collation choose a relation.
	var resolvedDatabase, resolvedTable string
	count := 0
	err := withFeatureRows(c.ctx, c.db, "SELECT TABLE_SCHEMA,TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?",
		[]any{database, table}, func(rows *sql.Rows) error {
			for rows.Next() {
				var dbName, tableName string
				if err := rows.Scan(&dbName, &tableName); err != nil {
					return err
				}
				match := dbName == database && tableName == table
				if c.lowerCaseTables != 0 {
					match = strings.EqualFold(dbName, database) && strings.EqualFold(tableName, table)
				}
				if !match {
					continue
				}
				count++
				resolvedDatabase, resolvedTable = dbName, tableName
			}
			return nil
		})
	if err != nil {
		return featuresql.RelationMetadata{}, err
	}
	if count != 1 {
		return featuresql.RelationMetadata{}, featureUnsupported("relation not uniquely visible")
	}
	schema, err := inspectFeatureSchema(c.ctx, c.db, resolvedDatabase, resolvedTable, c.allowLegacyTableIdentity)
	if err != nil {
		return featuresql.RelationMetadata{}, err
	}
	c.schema = schema
	return featuresql.RelationMetadata{Schema: resolvedDatabase, Name: resolvedTable,
		CatalogIdentity: schema.physicalID, PhysicalTable: true, Deterministic: true}, nil
}

func (c *mysqlFeatureCatalog) ResolveColumn(relation featuresql.RelationMetadata, identifier featuresql.Identifier) (featuresql.ColumnMetadata, error) {
	if relation.Schema != c.schema.database || relation.Name != c.schema.table {
		return featuresql.ColumnMetadata{}, featureUnsupported("column relation changed")
	}
	column, ok := c.schema.column(identifier.Name())
	if !ok {
		return featuresql.ColumnMetadata{}, featureInvalid("feature_sql", "unknown source column")
	}
	meta := featureColumnMetadata(column)
	for i, index := range c.schema.indexes {
		if index.nonUnique != 0 || index.position != 1 || index.column != column.name || index.prefix.Valid {
			continue
		}
		if i+1 < len(c.schema.indexes) && c.schema.indexes[i+1].name == index.name {
			continue
		}
		meta.SingleColumnUnique = true
	}
	return meta, nil
}

func (c *mysqlFeatureCatalog) OutputKey(identifier featuresql.Identifier) (string, error) {
	return strings.ToLower(identifier.Name()), nil
}
func (c *mysqlFeatureCatalog) QualifierKey(identifier featuresql.Identifier) (string, error) {
	if c.lowerCaseTables == 0 {
		return identifier.Name(), nil
	}
	return strings.ToLower(identifier.Name()), nil
}

func featureColumnMetadata(column featureColumn) featuresql.ColumnMetadata {
	bits, unsigned := integralColumn(column)
	meta := featuresql.ColumnMetadata{Name: column.name, Bits: bits, Unsigned: unsigned,
		Nullable: column.nullable == "YES", Generated: generatedColumn(column), Collation: column.collation}
	if bits != 0 {
		meta.Kind = featuresql.IntegerColumn
		return meta
	}
	switch strings.ToLower(column.dataType) {
	case "decimal", "numeric":
		meta.Kind = featuresql.DecimalColumn
		open := strings.IndexByte(column.columnType, '(')
		close := strings.IndexByte(column.columnType, ')')
		if open >= 0 && close > open {
			parts := strings.Split(column.columnType[open+1:close], ",")
			if len(parts) == 2 {
				meta.Precision, _ = strconv.Atoi(parts[0])
				meta.Scale, _ = strconv.Atoi(parts[1])
			}
		}
	case "float", "double", "real":
		meta.Kind = featuresql.FloatColumn
	case "char", "varchar", "tinytext", "text", "mediumtext", "longtext", "enum", "set":
		meta.Kind = featuresql.StringColumn
	case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob", "bit":
		meta.Kind = featuresql.BinaryColumn
	case "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection":
		meta.Kind = featuresql.NativeGeometryColumn
	default:
		meta.Kind = featuresql.OtherColumn
	}
	return meta
}

func (p *Provider) registerFeatureSelection(layer Layer, conf dict.Dicter, plan *featuresql.Plan) (*featureProfile, error) {
	ctx, cancel := codec.NewInspectionContext()
	defer cancel()
	catalog := &mysqlFeatureCatalog{ctx: ctx, db: p.db, database: p.Database, allowLegacyTableIdentity: p.allowLegacyTableIdentity}
	if err := p.db.QueryRowContext(ctx, "SELECT @@lower_case_table_names").Scan(&catalog.lowerCaseTables); err != nil {
		return nil, err
	}
	if catalog.lowerCaseTables < 0 || catalog.lowerCaseTables > 2 {
		return nil, featureUnsupported("table-name rules")
	}
	mapping, _, err := featureTemporalConfig(conf)
	if err != nil {
		return nil, err
	}
	temporal := []string{}
	for _, output := range []string{mapping.InstantField, mapping.StartField, mapping.EndField} {
		if output != "" {
			temporal = append(temporal, strings.ToLower(output))
		}
	}
	resolved, err := featuresql.Resolve(plan, catalog, featuresql.ResolveOptions{
		IdentityOutput: strings.ToLower(layer.idFieldname), GeometryOutput: strings.ToLower(layer.geomFieldname), TemporalOutputs: temporal})
	if err != nil {
		return nil, featureSQLAdmissionError(err)
	}
	profile := &featureProfile{publicTemporalNulls: true, schema: catalog.schema, id: resolved.Identity().Output, geometry: resolved.Geometry().Output,
		format: layer.geometryFormat, srid: layer.srid, mos: layer.mosConfig}
	id := resolved.Identity().Column
	profile.idBits, profile.idUnsigned = id.Bits, id.Unsigned
	for _, projection := range resolved.Projections() {
		profile.projection = append(profile.projection, featureProjection{source: projection.Column.Name, output: projection.Output})
		private := projection.Column.Name == resolved.Identity().Column.Name || projection.Column.Name == resolved.Geometry().Column.Name || featurePrivateColumn(layer, projection.Column.Name)
		if !private {
			profile.properties = append(profile.properties, projection.Output)
		}
	}
	// Native auto format must come from the proven raw relation, not tile SQL
	// sampling. Canonical MapplGIS evidence may establish MOS independently.
	mInfo, err := profile.detectFeatureMapplGIS(ctx, p.db)
	if err != nil {
		return nil, err
	}
	if mInfo.IsMapplGIS {
		copyLayer := layer
		copyLayer.mapplSource, copyLayer.mapplSysInfo = codec.MapplGISTableCanonical, mInfo.SystemInfo
		if err := applySystemInfo(&copyLayer, &copyLayer.mapplSysInfo, layer.crsExplicit); err != nil {
			return nil, err
		}
		profile.mos, profile.srid, profile.format = copyLayer.mosConfig, copyLayer.srid, copyLayer.geometryFormat
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
	profile.filter, profile.filterArgs, _, err = featuresql.CompileWhere(resolved, featuresql.RenderOptions{
		QuoteIdentifier: featureQuoteIdentifier, Placeholder: func(int) string { return "?" },
		FirstParameter: 1, Qualifier: "l", BindLiteral: bindFeatureLiteral, ParameterExpression: featureParameterExpression})
	if err != nil {
		return nil, featureSQLAdmissionError(err)
	}
	return profile, nil
}

func featureNumberLiteral(literal featuresql.Literal) (*big.Rat, error) {
	if literal.Kind() != featuresql.NumberLiteral {
		return nil, featureUnsupported("numeric predicate literal type")
	}
	text := literal.Text()
	if len(text) > 512 {
		return nil, featureUnsupported("numeric literal precision bound")
	}
	if e := strings.IndexAny(text, "eE"); e >= 0 {
		exponent, err := strconv.Atoi(text[e+1:])
		if err != nil || exponent < -65 || exponent > 65 {
			return nil, featureUnsupported("numeric literal exponent bound")
		}
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, featureInvalid("feature_sql", "invalid number")
	}
	return number, nil
}

func bindFeatureLiteral(column featuresql.ColumnMetadata, operator string, literal featuresql.Literal) (any, error) {
	switch column.Kind {
	case featuresql.IntegerColumn:
		var number *big.Rat
		var err error
		if literal.Kind() == featuresql.BooleanLiteral {
			number = new(big.Rat)
			if literal.Text() == "true" {
				number.SetInt64(1)
			}
		} else {
			number, err = featureNumberLiteral(literal)
		}
		if err != nil {
			return nil, err
		}
		if !number.IsInt() || column.Bits < 1 || column.Bits > 64 {
			return nil, featureUnsupported("integer literal representation")
		}
		value := number.Num()
		if column.Unsigned {
			if value.Sign() < 0 || value.BitLen() > column.Bits {
				return nil, featureUnsupported("unsigned predicate literal range")
			}
			return value.Uint64(), nil
		}
		bound := new(big.Int).Lsh(big.NewInt(1), uint(column.Bits-1))
		minimum := new(big.Int).Neg(new(big.Int).Set(bound))
		if value.Cmp(minimum) < 0 || value.Cmp(bound) >= 0 {
			return nil, featureUnsupported("signed predicate literal range")
		}
		return value.Int64(), nil
	case featuresql.DecimalColumn:
		if column.Precision < 1 || column.Precision > 65 || column.Scale < 0 || column.Scale > 38 || column.Scale > column.Precision {
			return nil, featureUnsupported("decimal catalog profile")
		}
		number, err := featureNumberLiteral(literal)
		if err != nil {
			return nil, err
		}
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(column.Scale)), nil)
		scaled := new(big.Rat).Mul(number, new(big.Rat).SetInt(factor))
		bound := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(column.Precision)), nil)
		if !scaled.IsInt() || new(big.Int).Abs(scaled.Num()).Cmp(bound) >= 0 || (column.Unsigned && number.Sign() < 0) {
			return nil, featureUnsupported("decimal literal not exactly representable")
		}
		return number.FloatString(column.Scale), nil
	case featuresql.StringColumn:
		if literal.Kind() != featuresql.StringLiteral || !utf8.ValidString(literal.Text()) {
			return nil, featureUnsupported("string predicate literal type")
		}
		if column.Collation != "utf8mb4_bin" && column.Collation != "utf8mb3_bin" && column.Collation != "utf8_bin" && column.Collation != "ascii_bin" {
			return nil, featureUnsupported("string predicate collation")
		}
		if strings.HasPrefix(column.Collation, "ascii_") {
			for _, r := range literal.Text() {
				if r > 127 {
					return nil, featureUnsupported("string predicate character set")
				}
			}
		}
		if column.Collation == "utf8_bin" || column.Collation == "utf8mb3_bin" {
			for _, r := range literal.Text() {
				if r > 0xffff {
					return nil, featureUnsupported("string predicate character set")
				}
			}
		}
		return literal.Text(), nil
	case featuresql.BinaryColumn:
		if literal.Kind() != featuresql.StringLiteral {
			return nil, featureUnsupported("binary predicate literal type")
		}
		return []byte(literal.Text()), nil
	default:
		return nil, featureUnsupported("predicate column type")
	}
}

func featureParameterExpression(column featuresql.ColumnMetadata, operator, placeholder string) (string, error) {
	switch column.Kind {
	case featuresql.IntegerColumn:
		if column.Bits < 1 || column.Bits > 64 {
			return "", featureUnsupported("integer cast profile")
		}
		if column.Unsigned {
			return "CAST(" + placeholder + " AS UNSIGNED)", nil
		}
		return "CAST(" + placeholder + " AS SIGNED)", nil
	case featuresql.DecimalColumn:
		if column.Precision < 1 || column.Precision > 65 || column.Scale < 0 || column.Scale > 38 || column.Scale > column.Precision {
			return "", featureUnsupported("decimal cast profile")
		}
		return "CAST(" + placeholder + " AS DECIMAL(" + strconv.Itoa(column.Precision) + "," + strconv.Itoa(column.Scale) + "))", nil
	case featuresql.BinaryColumn:
		return "CAST(" + placeholder + " AS BINARY)", nil
	case featuresql.StringColumn:
		return placeholder, nil
	default:
		return "", featureUnsupported("predicate cast profile")
	}
}

var _ featuresql.CatalogResolver = (*mysqlFeatureCatalog)(nil)

func (f *featureProfile) detectFeatureMapplGIS(ctx context.Context, db *sql.DB) (mapplgis.Info, error) {
	id, _ := f.sourceColumn(f.id)
	geometry, _ := f.sourceColumn(f.geometry)
	if !strings.EqualFold(id, mapplgis.PrimaryKey) || !strings.EqualFold(geometry, mapplgis.GeometryField) {
		return mapplgis.Info{}, nil
	}
	meta := mapplgis.TableMeta{}
	for _, column := range f.schema.columns {
		meta.Columns = append(meta.Columns, column.name)
	}
	for _, index := range f.schema.indexes {
		if index.name == "PRIMARY" {
			meta.PrimaryKeyColumns = append(meta.PrimaryKeyColumns, index.column)
		}
		if len(meta.Indexes) == 0 || meta.Indexes[len(meta.Indexes)-1].Name != index.name {
			meta.Indexes = append(meta.Indexes, mapplgis.IndexMeta{Name: index.name})
		}
		last := &meta.Indexes[len(meta.Indexes)-1]
		last.Columns = append(last.Columns, index.column)
	}
	return mapplgis.Detect(meta, func() (*mos.SystemInfo, error) {
		var blob []byte
		err := db.QueryRowContext(ctx, "SELECT "+featureQuoteIdentifier(mapplgis.GeometryField)+" FROM "+f.relation()+
			" WHERE "+featureQuoteIdentifier(mapplgis.PrimaryKey)+"=?", int64(1)).Scan(&blob)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		info, err := mos.ParseSystemInfo(blob)
		return &info, err
	})
}
