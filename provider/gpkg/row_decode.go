//go:build cgo

package gpkg

import (
	"context"
	"encoding/base64"
	"github.com/alexeydott/geom"
	"strings"
	"time"

	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// decodedRow retains representation evidence independently of tile policy.
type decodedRow struct {
	Feature           provider.Feature
	Representable     bool
	HadAbsentGeometry bool
}

// rowDecodePolicy separates raw-feature integrity from the legacy tile policy.
// Its zero value is strict: malformed geometry is returned as an error.
type rowDecodePolicy struct {
	tolerateAutoDetectedMOS bool
	dimensionalRaw          bool
}

// decodeRow shares row construction without tile/query filtering. A false
// Representable result identifies metadata or rows excluded by existing ID/MOS
// policies. Null and empty geometry remain representable; the tile caller owns
// their exclusion using sticky HadAbsentGeometry evidence, including duplicate
// geometry aliases. Inputs and the resolved layer CRS/MOS configuration stay fixed.
func (p *Provider) decodeRow(
	ctx context.Context,
	layer *Layer,
	columns []string,
	values []any,
	policy rowDecodePolicy,
) (decodedRow, error) {
	var err error
	feature := provider.Feature{
		Tags: map[string]interface{}{},
	}
	log.Logger().Debug("decoding gpkg feature row", "column_count", len(columns))
	defer log.Logger().Debug("gpkg feature row decoding finished")
	if err := ctx.Err(); err != nil {
		return decodedRow{}, err
	}
	skipRow := false
	hadAbsentGeometry := false

	for i := range columns {
		if err := ctx.Err(); err != nil {
			return decodedRow{}, err
		}
		if values[i] == nil {
			// P6-17: column-name lookups are case-insensitive because
			// SQLite column names can differ in case from the
			// configured names.
			if strings.EqualFold(columns[i], layer.geomFieldname) {
				hadAbsentGeometry = true
				continue
			}
			if strings.EqualFold(columns[i], layer.idFieldname) {
				// P6-11: a NULL feature id would silently become ID 0 and
				// collapse distinct features into one in the MVT.
				// provider.Feature.ID is a plain uint64, so a feature
				// without an id cannot be represented; skip the row and
				// warn instead (documented choice).
				p.warnOnce("null-feature-id:"+layer.name,
					"gpkg layer '%v': NULL feature id in column %q; skipping row",
					layer.name, layer.idFieldname)
				skipRow = true
			}
			continue
		}

		switch {
		case strings.EqualFold(columns[i], layer.idFieldname):
			feature.ID, err = provider.ConvertFeatureID(values[i])
			if err != nil {
				return decodedRow{}, err
			}

		case strings.EqualFold(columns[i], layer.geomFieldname):
			// The MOS layer self-description blob (MapplGIS LayerInfo)
			// is metadata, never a feature. System-info parameters are
			// finalized at registration (canonical one-time detection,
			// see gpkg_register.go detectMapplGIS); applying them
			// mid-stream would decode earlier rows with different
			// precision/units and invalidate the already-built SQL
			// bounds filter, so late blobs are skipped (R6).
			if layer.geometryFormat == codec.FormatMOS && codec.IsSystemInfoValue(values[i]) {
				p.warnOnce("late-system-info:"+layer.name,
					"layer '%v': MOS system-info row encountered after the query was built; skipping",
					layer.name)
				skipRow = true
				continue
			}

			var geo geom.Geometry
			var err error
			if policy.dimensionalRaw {
				geo, err = decodeRawGeometryValue(values[i], layer)
			} else {
				_, geo, err = decodeGeometryValue(values[i], layer.geometryFormat, layer.mosConfig)
			}
			if err != nil {
				if policy.tolerateAutoDetectedMOS && layer.mapplSource == codec.MapplGISSQLSample {
					log.Warnf("layer %v: skipping undecodable auto-detected MOS geometry: %v", layer.name, err)
					skipRow = true
					break
				}
				log.Errorf("error decoding geometry: %v", err)
				return decodedRow{}, err
			}
			if geo == nil {
				hadAbsentGeometry = true
				// Preserve empty geometry for the caller's representation
				// policy, and continue validating the remaining columns.
				continue
			}

			// The layer SRID is resolved at registration time from the
			// CRS contract (explicit config > gpkg_contents.srs_id >
			// provider default, with MOS system-info projection as a
			// last step), so the per-row WKB header is not consulted
			// here.
			// mixed-content policy for explicit geometry_type: permit
			// the feature but warn once per layer.
			if layer.geomTypeExplicit {
				codec.WarnOnceGeometryTypeMismatch(layer.Name(), layer.geomType, geo)
			}
			feature.Geometry = geo
		default:
			// Bounds fields backing the SQL bounds filter (detected
			// raw bounds columns for tablename layers, resolved
			// bbox_*_fieldname for custom SQL) are operational
			// columns, not user attributes: never leak them into
			// feature tags.
			if layer.bboxFields.IsBBoxField(columns[i]) {
				continue
			}
			// Legacy fixed zoom-filter columns keep their exclusion.
			// Bounds columns are excluded solely through
			// bboxFields.IsBBoxField above (the resolved
			// bbox_*_fieldname contract): a column merely NAMED
			// minx/miny/maxx/maxy that is not a bounds field is an
			// ordinary user tag (audit 7.2.5).
			switch strings.ToLower(columns[i]) {
			case "min_zoom", "max_zoom":
				// Skip these columns used for zoom filtering
				continue
			}
			// Grab any non-nil, non-id, non-bounding box, & non-geometry column as a tag
			switch v := values[i].(type) {
			case []uint8:
				// P6-18: BLOB tag values are arbitrary binary and
				// string(v) would emit invalid UTF-8 strings in the
				// MVT. Encode them as standard base64 (lossless and
				// cheap) so tags remain valid strings (documented
				// choice over dropping the tag).
				feature.Tags[columns[i]] = base64.StdEncoding.EncodeToString(v)
			case string:
				feature.Tags[columns[i]] = v
			case int64:
				feature.Tags[columns[i]] = v
			case float64:
				feature.Tags[columns[i]] = v
			case bool:
				feature.Tags[columns[i]] = v
			case time.Time:
				feature.Tags[columns[i]] = v.Format(time.RFC3339)
			default:
				// Emit a warning once per column: the same unexpected
				// type recurs for every row of the stream.
				p.warnOnce("unexpected-column:"+columns[i],
					"unexpected type for sqlite column data: %v: %T", columns[i], v)
			}
		}
	}

	feature.SRID = layer.srid
	if feature.SRID == 0 {
		feature.SRID = DefaultSRID
	}
	return decodedRow{Feature: feature, Representable: !skipRow, HadAbsentGeometry: hadAbsentGeometry}, nil
}
