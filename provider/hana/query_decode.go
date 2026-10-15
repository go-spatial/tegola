package hana

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/SAP/go-hdb/driver"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

const featureRowByteLimit = 64 << 20

type featureBoundedWriter struct {
	bytes.Buffer
	remaining *int
}

func (w *featureBoundedWriter) Write(value []byte) (int, error) {
	if len(value) > *w.remaining {
		return 0, featureUnsupported("row payload exceeds bounded profile")
	}
	*w.remaining -= len(value)
	return w.Buffer.Write(value)
}

func quoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func featureSourceError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return provider.FeatureDataError{Err: err}
}

func featureScanRow(rows *sql.Rows, columns []featureColumn) (map[string]any, error) {
	remaining := featureRowByteLimit
	targets := make([]any, len(columns))
	for i, column := range columns {
		switch column.Type {
		case "BLOB", "CLOB", "NCLOB", "TEXT", "ST_POINT", "ST_GEOMETRY":
			targets[i] = &driver.NullLob{Lob: new(driver.Lob).SetWriter(&featureBoundedWriter{remaining: &remaining})}
		case "DECIMAL", "SMALLDECIMAL":
			targets[i] = &driver.NullDecimal{Decimal: new(driver.Decimal)}
		default:
			targets[i] = new(any)
		}
	}
	if err := rows.Scan(targets...); err != nil {
		return nil, err
	}
	values := make(map[string]any, len(columns))
	for i, column := range columns {
		var value any
		switch target := targets[i].(type) {
		case *driver.NullLob:
			if target.Valid {
				writer := target.Lob.Writer().(*featureBoundedWriter)
				value = append([]byte(nil), writer.Bytes()...)
				if column.Type != "BLOB" && column.Type != "ST_POINT" && column.Type != "ST_GEOMETRY" {
					value = string(value.([]byte))
				}
			}
		case *driver.NullDecimal:
			if target.Valid {
				if column.Type == "SMALLDECIMAL" {
					return nil, featureUnsupported("SMALLDECIMAL exact property representation unproven")
				}
				rational := (*big.Rat)(target.Decimal)
				if column.Scale < 0 || column.Scale > 38 || column.Length <= 0 || column.Length > 38 {
					return nil, featureInvalid("property", "decimal metadata invalid")
				}
				scaled := new(big.Rat).Mul(rational, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(column.Scale), nil)))
				if !scaled.IsInt() || len(new(big.Int).Abs(scaled.Num()).String()) > int(column.Length) {
					return nil, featureInvalid("property", "decimal value exceeds declared precision or scale")
				}
				value = json.Number(rational.FloatString(int(column.Scale)))
			}
		case *any:
			value = *target
			if binary, ok := value.([]byte); ok {
				if len(binary) > remaining {
					return nil, featureUnsupported("row payload exceeds bounded profile")
				}
				remaining -= len(binary)
				switch column.Type {
				case "VARCHAR", "NVARCHAR", "CHAR", "NCHAR":
					if !utf8.Valid(binary) {
						return nil, featureInvalid("property", "invalid UTF-8 source text")
					}
					value = string(binary)
				default:
					value = append([]byte(nil), binary...)
				}
			}
			if text, ok := value.(string); ok {
				if len(text) > remaining {
					return nil, featureUnsupported("row payload exceeds bounded profile")
				}
				remaining -= len(text)
			}
		}
		if f, ok := value.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
			return nil, featureInvalid("property", "nonfinite source value")
		}
		values[column.Name] = value
	}
	return values, nil
}

func decodeFeature(l Layer, physical map[string]any, fields map[string]bool) (provider.Feature, bool, error) {
	s := l.feature
	values := make(map[string]any, len(s.Projections))
	for _, projection := range s.Projections {
		values[projection.Output] = physical[projection.Physical]
	}
	if values[s.ID] == nil {
		return provider.Feature{}, false, nil
	}
	id, ok := featureInteger(values[s.ID])
	if !ok || id < 0 {
		return provider.Feature{}, false, featureInvalid("id", "source identity is not a nonnegative integral value")
	}
	if err := validateTemporalValues(s, values); err != nil {
		return provider.Feature{}, false, err
	}
	feature := provider.Feature{ID: uint64(id), SRID: s.SRID, Tags: map[string]any{}}
	var err error
	if value := values[s.Geometry]; value != nil {
		if l.geometryFormat == "mos" && codec.IsSystemInfoValue(value) {
			return provider.Feature{}, false, nil
		}
		switch l.geometryFormat {
		case "wkb", "":
			feature.Geometry, err = codec.DecodeRawWKB(value)
		case "wkt":
			feature.Geometry, err = codec.DecodeRawWKT(value)
		case "mos":
			feature.Geometry, err = codec.DecodeMOS(value, l.mosConfig)
		default:
			err = featureUnsupported("geometry storage unsupported")
		}
	}
	if err != nil {
		return provider.Feature{}, false, err
	}
	if err := codec.ValidateFeatureSpatialGeometry(feature.Geometry); err != nil {
		return provider.Feature{}, false, err
	}
	if err := validateSourceDimension(feature.Geometry, s.Spatial.Dimension); err != nil {
		return provider.Feature{}, false, err
	}
	if codec.FeatureSpatialGeometryEmpty(feature.Geometry) {
		feature.Geometry = nil
	}
	for _, projection := range s.Projections {
		if s.Private[projection.Physical] || !fields[projection.Output] {
			continue
		}
		value := values[projection.Output]
		if _, err := json.Marshal(value); err != nil {
			return provider.Feature{}, false, fmt.Errorf("source property JSON representation: %w", err)
		}
		feature.Tags[projection.Output] = value
	}
	return feature, true, nil
}
