package hana

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Catalog snapshots freeze qualified physical identity, not sampled SQL labels.
// Unknown catalog versions/profiles fail admission rather than weakening proof.
type featureColumn struct {
	Name                             string
	ID                               int64
	Type                             string
	Length, Scale                    int64
	Nullable                         bool
	Collation, Generated, Generation string
	Hidden, Masked                   bool
}

type featureNativeColumn struct {
	Name       string
	SRID       uint64
	Dimensions int
	SRS        featureNativeSRS
}

type featureCatalog struct {
	Schema, Table                                     string
	TableType, SessionType, Temporary, Policy, Masked string
	Temporal                                          sql.NullString
	OID                                               int64
	Columns                                           []featureColumn
	UniqueKeys                                        [][]string
	Native                                            []featureNativeColumn
}

type featureCatalogReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readFeatureCatalog(ctx context.Context, db featureCatalogReader, schema, table string) (featureCatalog, error) {
	c := featureCatalog{Schema: schema, Table: table}
	if c.Schema == "" {
		if err := db.QueryRowContext(ctx, "SELECT CURRENT_SCHEMA FROM DUMMY").Scan(&c.Schema); err != nil {
			return c, fmt.Errorf("HANA feature current schema: %w", err)
		}
	}
	var typ, temp, policy, masked, session string
	var temporal sql.NullString
	err := db.QueryRowContext(ctx, `SELECT TABLE_OID,TABLE_TYPE,IS_TEMPORARY,HAS_STRUCTURED_PRIVILEGE_CHECK,HAS_MASKED_COLUMNS,TEMPORAL_TYPE,SESSION_TYPE FROM SYS.TABLES WHERE SCHEMA_NAME=? AND TABLE_NAME=?`, c.Schema, c.Table).Scan(&c.OID, &typ, &temp, &policy, &masked, &temporal, &session)
	if err != nil {
		return c, fmt.Errorf("HANA feature physical table: %w", err)
	}
	if c.OID <= 0 || (typ != "ROW" && typ != "COLUMN") || temp != "FALSE" || policy != "FALSE" || masked != "FALSE" || temporal.String != "" || (session != "NONE" && !(typ == "COLUMN" && session == "SIMPLE")) {
		return c, fmt.Errorf("HANA feature table has unsupported persistence, visibility or temporal semantics")
	}
	c.TableType, c.SessionType, c.Temporary, c.Policy, c.Masked, c.Temporal = typ, session, temp, policy, masked, temporal
	if err := readFeatureColumns(ctx, db, &c); err != nil {
		return c, err
	}
	if err := readFeatureUniqueKeys(ctx, db, &c); err != nil {
		return c, err
	}
	for _, col := range c.Columns {
		if col.Type != "ST_POINT" && col.Type != "ST_GEOMETRY" {
			continue
		}
		var spatial featureNativeColumn
		spatial.Name = col.Name
		if err := db.QueryRowContext(ctx, "SELECT SRS_ID,COORD_DIMENSION FROM SYS.ST_GEOMETRY_COLUMNS WHERE SCHEMA_NAME=? AND TABLE_NAME=? AND COLUMN_NAME=?", c.Schema, c.Table, col.Name).Scan(&spatial.SRID, &spatial.Dimensions); err != nil {
			return c, fmt.Errorf("HANA native geometry catalog: %w", err)
		}
		if err := db.QueryRowContext(ctx, "SELECT DEFINITION,TRANSFORM_DEFINITION,ROUND_EARTH,ORGANIZATION,ORGANIZATION_COORDSYS_ID FROM SYS.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_ID=?", spatial.SRID).Scan(&spatial.SRS.Definition, &spatial.SRS.Transform, &spatial.SRS.RoundEarth, &spatial.SRS.Organization, &spatial.SRS.OrganizationID); err != nil {
			return c, fmt.Errorf("HANA native source CRS fingerprint: %w", err)
		}
		c.Native = append(c.Native, spatial)
	}
	return c, nil
}

func readFeatureColumns(ctx context.Context, db featureCatalogReader, c *featureCatalog) (err error) {
	rows, err := db.QueryContext(ctx, `SELECT COLUMN_NAME,COLUMN_ID,DATA_TYPE_NAME,LENGTH,SCALE,IS_NULLABLE,COLLATION,GENERATED_ALWAYS_AS,GENERATION_TYPE,IS_HIDDEN,IS_MASKED FROM SYS.TABLE_COLUMNS WHERE SCHEMA_NAME=? AND TABLE_NAME=? ORDER BY POSITION`, c.Schema, c.Table)
	if err != nil {
		return fmt.Errorf("HANA feature columns: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	seen := map[string]bool{}
	for rows.Next() {
		var col featureColumn
		var nullable, hidden, masked string
		var length, scale sql.NullInt64
		var collation, generated, generation sql.NullString
		if err := rows.Scan(&col.Name, &col.ID, &col.Type, &length, &scale, &nullable, &collation, &generated, &generation, &hidden, &masked); err != nil {
			return err
		}
		if seen[col.Name] || col.Name == "" || col.ID <= 0 || len(c.Columns) >= 1024 {
			return fmt.Errorf("HANA feature column metadata is ambiguous or exceeds profile")
		}
		if (nullable != "TRUE" && nullable != "FALSE") || (hidden != "TRUE" && hidden != "FALSE") || (masked != "TRUE" && masked != "FALSE") {
			return fmt.Errorf("HANA feature column flags are unknown")
		}
		col.Length, col.Scale = length.Int64, scale.Int64
		col.Nullable, col.Hidden, col.Masked = nullable == "TRUE", hidden == "TRUE", masked == "TRUE"
		col.Collation, col.Generated, col.Generation = collation.String, generated.String, generation.String
		seen[col.Name] = true
		c.Columns = append(c.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(c.Columns) == 0 {
		return fmt.Errorf("HANA feature source has no proven columns")
	}
	return nil
}

func readFeatureUniqueKeys(ctx context.Context, db featureCatalogReader, c *featureCatalog) (err error) {
	rows, err := db.QueryContext(ctx, `SELECT i.INDEX_NAME,ic.COLUMN_NAME FROM SYS.INDEXES i JOIN SYS.INDEX_COLUMNS ic ON i.SCHEMA_NAME=ic.SCHEMA_NAME AND i.TABLE_NAME=ic.TABLE_NAME AND i.INDEX_NAME=ic.INDEX_NAME WHERE i.SCHEMA_NAME=? AND i.TABLE_NAME=? AND i.TABLE_OID=? AND i.CONSTRAINT IN ('PRIMARY KEY','PRIMARY_KEY','UNIQUE','NOT_NULL_UNIQUE') ORDER BY i.INDEX_NAME,ic.POSITION`, c.Schema, c.Table, c.OID)
	if err != nil {
		return fmt.Errorf("HANA feature unique constraints: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	previous := ""
	for rows.Next() {
		var index, column string
		if err := rows.Scan(&index, &column); err != nil {
			return err
		}
		if index == "" || column == "" {
			return fmt.Errorf("HANA feature key lineage unknown")
		}
		if index != previous {
			c.UniqueKeys = append(c.UniqueKeys, []string{})
			previous = index
		}
		last := len(c.UniqueKeys) - 1
		c.UniqueKeys[last] = append(c.UniqueKeys[last], column)
	}
	return rows.Err()
}

func (c featureCatalog) column(name string) (featureColumn, bool) {
	for _, col := range c.Columns {
		if col.Name == name {
			return col, true
		}
	}
	return featureColumn{}, false
}

func (c featureCatalog) uniqueInteger(name string) bool {
	col, ok := c.column(name)
	if !ok || !featureIntegral(col.Type) || featureComputed(col) || col.Hidden || col.Masked {
		return false
	}
	for _, key := range c.UniqueKeys {
		if len(key) == 1 && key[0] == name {
			return true
		}
	}
	return false
}

func featureIntegral(typ string) bool {
	switch typ {
	case "TINYINT", "SMALLINT", "INTEGER", "BIGINT":
		return true
	}
	return false
}

func featureComputed(c featureColumn) bool {
	if strings.TrimSpace(c.Generated) != "" {
		return true
	}
	switch c.Generation {
	case "", "ALWAYS AS IDENTITY", "BY DEFAULT AS IDENTITY":
		return false
	}
	return true
}

func (c featureCatalog) equal(other featureCatalog) bool { return reflect.DeepEqual(c, other) }

func (c featureCatalog) qualified() string {
	return quoteIdent(c.Schema) + "." + quoteIdent(c.Table)
}
