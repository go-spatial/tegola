package hana

import (
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// layer holds information about a query.
type Layer struct {
	// The Name of the layer.
	name string
	// The SQL to use when querying HANA for this layer.
	sql string
	// The ID field name, this will default to 'gid' if not set to something other then empty string.
	idField string
	// The Geometery field name, this will default to 'geom' if not set to something other then empty string.
	geomField string
	// GeomType is the the type of geometry returned from the SQL.
	geomType geom.Geometry
	// The SRID that the data in the table is stored in. This will default to WebMercator.
	srid uint64
	// The description of fields in the sql query. Used only by the non-MVT provider.
	fields []FieldDescription
	// geometryFormat selects how the geometry column is decoded:
	// "" (default; HANA native geometry returned via ST_AsBinary),
	// "wkb" (plain WKB column read raw, no ST_AsBinary), "wkt" (WKT text)
	// or "mos" (opaque MapplGIS binary blob read raw).
	geometryFormat string
	// mosConfig holds the resolved MOS quantization settings used with
	// geometry_format = "mos".
	mosConfig codec.MOSConfig
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
	// bboxTable qualifies custom tile SQL bounds, never feature identity.
	bboxTable    string
	bboxFields   codec.BBoxFields
	featureTable string
	featureSRID  uint64
	feature      *featureSource
	featureError error
}

func (l Layer) FeatureQuerySupported() error {
	if l.feature == nil {
		return featureUnsupported("metadata is unavailable")
	}
	if l.featureError != nil {
		return l.featureError
	}
	return nil
}
func (l Layer) TemporalMapping() (provider.TemporalMapping, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.TemporalMapping{}, err
	}
	return l.feature.Temporal, nil
}
func (l Layer) SpatialMetadata() (provider.SpatialMetadata, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.SpatialMetadata{}, err
	}
	return l.feature.Spatial, nil
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

// FeatureSourceSRID retains the physical source CRS independently of the
// planar-equivalent label used by the tile query.
func (l Layer) FeatureSourceSRID() uint64 {
	if l.feature == nil {
		return 0
	}
	return l.feature.SRID
}

func (l Layer) GeomFieldName() string {
	return l.geomField
}

func (l Layer) IDFieldName() string {
	return l.idField
}

// MapplGISSource reports the detection source. SystemInfo is only guaranteed
// for MapplGISTableCanonical.
func (l Layer) MapplGISSource() codec.MapplGISSource { return l.mapplSource }

func (l Layer) FieldDescriptions() []FieldDescription {
	return l.fields
}
