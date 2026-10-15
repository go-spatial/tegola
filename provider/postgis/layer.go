package postgis

import (
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// layer holds information about a query.
type Layer struct {
	feature *featureProfile
	// The Name of the layer
	name string
	// The SQL to use when querying PostGIS for this layer
	sql string
	// tablename is the configured table name (empty for custom SQL layers)
	tablename string
	// The ID field name, this will default to 'gid' if not set to something other then empty string.
	idField string
	// The Geometery field name, this will default to 'geom' if not set to something other then empty string.
	geomField string
	// GeomType is the the type of geometry returned from the SQL
	geomType geom.Geometry
	// The SRID that the data in the table is stored in. This will default to WebMercator
	srid uint64
	// geometryFormat selects how the geometry column is decoded:
	// "" (default; PostGIS native geometry returned via ST_AsBinary),
	// "wkb" (plain WKB column read raw, no ST_AsBinary), "wkt" (WKT text)
	// or "mos" (opaque MapplGIS binary blob read raw).
	geometryFormat string
	// mosConfig holds the resolved MOS quantization settings used with
	// geometry_format = "mos".
	mosConfig codec.MOSConfig
	// crsExplicit reports whether the layer's CRS was configured explicitly
	// (via srid or crs_defn at the provider or layer level). When false, a
	// source-provided projection (MOS system-info blob) may override srid.
	crsExplicit bool
	// isMapplGIS reports that the backing table satisfied the canonical
	// MapplGIS table contract at registration (provider/mapplgis). Only
	// set for tablename layers; custom SQL is never auto-detected.
	isMapplGIS bool
	// mapplSource distinguishes how the layer was detected as MapplGIS:
	// canonical table detection (SystemInfo guaranteed) or the SQL sample
	// probe (no SystemInfo; explicit CRS config required).
	mapplSource codec.MapplGISSource
	// mapplSysInfo is the parsed layer self-description of a detected
	// MapplGIS table. Valid only when mapplSource is MapplGISTableCanonical.
	mapplSysInfo mos.SystemInfo
	// bboxFields holds the resolved bounds field names (layer > provider >
	// defaults) backing the bounds-backed custom-SQL !BBOX! predicate for
	// raw (MOS) layers, and excluded from feature tags.
	bboxFields codec.BBoxFields
	// bboxTable qualifies bounds references in custom SQL, not result column names.
	bboxTable string
}

func (l Layer) Name() string {
	return l.name
}

func (l Layer) GeomType() geom.Geometry {
	return l.geomType
}

func (l Layer) SRID() uint64 {
	return l.srid
}

func (l Layer) GeomFieldName() string {
	return l.geomField
}

func (l Layer) IDFieldName() string {
	return l.idField
}

// MapplGISSource reports the detection source. SystemInfo is only guaranteed
// for MapplGISTableCanonical.
func (l Layer) MapplGISSource() codec.MapplGISSource {
	return l.mapplSource
}
