//go:build cgo

package gpkg

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func invalidQuery(field, reason string) error {
	return provider.InvalidFeatureQueryError{Field: field, Reason: reason}
}

func (p *Provider) registerFeatureQueries(configs []dict.Dicter) error {
	for _, conf := range configs {
		name, err := conf.String(ConfigKeyLayerName, nil)
		if err != nil {
			return err
		}
		layer := p.layers[name]
		if err := p.registerSpatial(layer, conf); err != nil {
			return err
		}
		layer.freezeFeatureCRS()
		mapping := provider.TemporalMapping{}
		keys := []string{"temporal_field", "temporal_start_field", "temporal_end_field", "temporal_storage"}
		values := make([]string, len(keys))
		present := make([]bool, len(keys))
		for i, key := range keys {
			value, err := conf.String(key, nil)
			var missing dict.ErrKeyRequired
			if errors.As(err, &missing) {
				continue
			}
			if err != nil || strings.TrimSpace(value) == "" {
				return invalidQuery("temporal", "configuration requires nonblank string values")
			}
			values[i], present[i] = value, true
		}
		mapping.InstantField, mapping.StartField, mapping.EndField = values[0], values[1], values[2]
		if err := mapping.Validate(); err != nil {
			return err
		}
		mapped := mapping != (provider.TemporalMapping{})
		if mapped != present[3] {
			return invalidQuery("temporal", "storage is required exactly when temporal fields are mapped")
		}
		if mapped {
			switch values[3] {
			case "unix_seconds":
				layer.temporalScale = 1
			case "unix_milliseconds":
				layer.temporalScale = 1000
			case "unix_microseconds":
				layer.temporalScale = 1000000
			case "unix_nanoseconds":
				layer.temporalScale = 1000000000
			default:
				return fmt.Errorf("gpkg temporal storage profile: %w", provider.ErrUnsupported)
			}
		}
		layer.temporalMapping = mapping
		if layer.tablename == "" {
			layer.featureQueryError = fmt.Errorf("gpkg custom SQL feature profile: %w", provider.ErrUnsupported)
			if mapped {
				return fmt.Errorf("gpkg custom SQL temporal profile: %w", provider.ErrUnsupported)
			}
			continue
		}
		cols, _, err := tableColumnsAndPK(p.db, layer.tablename)
		if err != nil {
			return err
		}
		layer.featureColumns = append([]string(nil), cols...)
		if err := resolveTemporalFields(layer, cols); err != nil {
			return err
		}
		if !uniqueIntegerID(p.db, layer.tablename, layer.idFieldname) {
			layer.featureQueryError = fmt.Errorf("gpkg feature profile requires a unique INTEGER identity: %w", provider.ErrUnsupported)
		}
		if (layer.geometryFormat == "" || layer.geometryFormat == GeometryFormatGPKG) && layer.tileQueryPlan == planRTree {
			// RTree keys reference the SQLite rowid, which need not equal a
			// separately configured unique feature identity. Avoid shadowed
			// hidden aliases and fail closed on WITHOUT ROWID sources.
			for _, alias := range []string{"_rowid_", "rowid", "oid"} {
				if containsFold(cols, alias) {
					continue
				}
				if accessibleRowID(p.db, layer.tablename, alias) {
					layer.featureRowIDAlias = alias
					break
				}
			}
			if layer.featureRowIDAlias == "" {
				layer.featureQueryError = fmt.Errorf("gpkg RTree source has no accessible rowid: %w", provider.ErrUnsupported)
			}
		}
		p.initializeFilterCatalog(layer)
	}
	return nil
}

func resolveTemporalFields(layer *Layer, columns []string) error {
	fields := []*string{&layer.temporalMapping.InstantField, &layer.temporalMapping.StartField, &layer.temporalMapping.EndField}
	for _, field := range fields {
		if *field == "" {
			continue
		}
		found := ""
		for _, col := range columns {
			if strings.EqualFold(col, *field) {
				if found != "" {
					return invalidQuery("temporal", "ambiguous source field")
				}
				found = col
			}
		}
		if found == "" {
			return invalidQuery("temporal", "source field does not exist")
		}
		*field = found
	}
	return layer.temporalMapping.Validate()
}

// Schema identity proof admits single-column INTEGER primary or unique keys.
// Nullable unique keys retain the existing skip policy for NULL identities.
func uniqueIntegerID(db *sql.DB, table, field string) bool {
	integer, pk, pkCount, err := inspectIntegerID(db, table, field)
	if err != nil || !integer {
		return false
	}
	if pk && pkCount == 1 {
		return true
	}
	names, err := uniqueIndexNames(db, table)
	if err != nil {
		return false
	}
	for _, name := range names {
		matches, err := indexHasSingleField(db, name, field)
		if err == nil && matches {
			return true
		}
	}
	return false
}

func accessibleRowID(db *sql.DB, table, alias string) (accessible bool) {
	rows, err := db.Query("SELECT " + quoteIdent(alias) + " FROM " + quoteIdent(table) + " LIMIT 0")
	if err != nil {
		return false
	}
	defer func() {
		if err := rows.Close(); err != nil {
			accessible = false
		}
	}()
	if rows.Next() {
		return false
	}
	return rows.Err() == nil
}

func inspectIntegerID(db *sql.DB, table, field string) (integer, pk bool, pkCount int, err error) {
	rows, err := db.Query("PRAGMA table_info(" + quoteIdent(table) + ")")
	if err != nil {
		return false, false, 0, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var cid, notnull, primary int
		var name, declaration string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &declaration, &notnull, &def, &primary); err != nil {
			return false, false, 0, err
		}
		if primary != 0 {
			pkCount++
		}
		if strings.EqualFold(name, field) {
			integer = strings.EqualFold(declaration, "INTEGER")
			pk = primary != 0
		}
	}
	return integer, pk, pkCount, rows.Err()
}

func uniqueIndexNames(db *sql.DB, table string) (names []string, err error) {
	indexes, err := db.Query("PRAGMA index_list(" + quoteIdent(table) + ")")
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, indexes.Close()) }()
	for indexes.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := indexes.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		if unique != 0 && partial == 0 {
			names = append(names, name)
		}
	}
	return names, indexes.Err()
}

func indexHasSingleField(db *sql.DB, index, field string) (matches bool, err error) {
	info, err := db.Query("PRAGMA index_info(" + quoteIdent(index) + ")")
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, info.Close()) }()
	count := 0
	for info.Next() {
		var seq, cid int
		var col sql.NullString
		if err := info.Scan(&seq, &cid, &col); err != nil {
			return false, err
		}
		count++
		matches = col.Valid && strings.EqualFold(col.String, field)
	}
	return count == 1 && matches, info.Err()
}
