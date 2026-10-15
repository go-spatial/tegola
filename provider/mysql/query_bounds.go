package mysql

import (
	"math"
	"strings"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// Bounds are an explicit source contract, never inferred from coincidental
// column names. Selection profiles retain physical column lineage.
func (f *featureProfile) registerFeatureBounds(layer Layer, conf dict.Dicter) {
	if f.format != GeometryFormatMOS {
		return
	}
	keys := [4]string{codec.ConfigKeyBBoxMinXField, codec.ConfigKeyBBoxMaxXField, codec.ConfigKeyBBoxMinYField, codec.ConfigKeyBBoxMaxYField}
	var fields [4]string
	seen := make(map[string]bool)
	geometry, ok := f.sourceColumn(f.geometry)
	if !ok {
		return
	}
	seen[strings.ToLower(geometry)] = true
	seen[strings.ToLower(f.physicalID())] = true
	for i, key := range keys {
		if _, explicit := conf.Interface(key); !explicit {
			return
		}
		column, ok := f.schema.column(layer.bboxFields[i])
		name := strings.ToLower(column.name)
		if !ok || seen[name] {
			return
		}
		switch strings.ToLower(column.dataType) {
		case "tinyint", "smallint", "mediumint", "int", "integer", "bigint", "decimal", "numeric", "double":
		default:
			return
		}
		seen[name], fields[i] = true, column.name
	}
	f.bounds = fields
}

// Only an identical pinned definition permits SQL bounds pruning. An equal
// synthetic number alone does not establish equal coordinate systems.
func (f *featureProfile) boundsCandidate(query provider.FeatureQuery) (string, []any) {
	if f.format != GeometryFormatMOS || f.bounds[0] == "" || len(query.Bounds) == 0 || len(query.Bounds3D) != 0 {
		return "", nil
	}
	if query.BoundsCRSDefinition != "" {
		if f.crsProjection == nil || query.BoundsCRSDefinition != f.crs.Definition {
			return "", nil
		}
	} else if query.BoundsSRID != f.srid || basic.IsSyntheticSRID(f.srid) || (f.crs.Definition != "" && f.crs.CanonicalAuthority == "") {
		return "", nil
	}
	scale, err := codec.MOSRawScale(f.mos)
	if err != nil || scale <= 0 || math.IsInf(scale, 0) || math.IsNaN(scale) {
		return "", nil
	}
	fields := [4]string{}
	for i, name := range f.bounds {
		fields[i] = "l." + featureQuoteIdentifier(name)
	}
	var clauses []string
	var args []any
	for _, box := range query.Bounds {
		v := [4]float64{math.Floor(box[0] * scale), math.Ceil(box[2] * scale), math.Floor(box[1] * scale), math.Ceil(box[3] * scale)}
		for _, n := range v {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return "", nil
			}
		}
		clauses = append(clauses, "("+fields[1]+">=? AND "+fields[0]+"<=? AND "+fields[3]+">=? AND "+fields[2]+"<=?)")
		args = append(args, v[0], v[1], v[2], v[3])
	}
	// Unclassifiable/absent geometry and missing or invalid bounds must reach
	// the normal decoder rather than disappear through SQL NULL comparisons.
	for _, field := range fields {
		clauses = append(clauses, field+" IS NULL")
	}
	clauses = append(clauses, fields[0]+">"+fields[1], fields[2]+">"+fields[3])
	geometry, ok := f.sourceColumn(f.geometry)
	if !ok {
		return "", nil
	}
	g := "l." + featureQuoteIdentifier(geometry)
	clauses = append(clauses, g+" IS NULL", "LENGTH("+g+")<18", "HEX(SUBSTRING("+g+",1,1)) NOT IN ('00','01','02','03','04')", "HEX(SUBSTRING("+g+",5,2))='0000'", "HEX(SUBSTRING("+g+",7,4))='00000000'")
	return "(" + strings.Join(clauses, " OR ") + ")", args
}
