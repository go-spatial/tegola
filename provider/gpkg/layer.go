package gpkg

import (
	"errors"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

type Layer struct {
	name          string
	tablename     string
	tagFieldnames []string
	idFieldname   string
	geomFieldname string
	geomType      geom.Geometry
	// geomTypeExplicit marks that geometry_type was set explicitly in the
	// layer config: the declared type wins over sampled/metadata inference
	// and runtime features of a different type are permitted with a
	// one-time warning.
	geomTypeExplicit bool
	srid             uint64
	bbox             geom.Extent
	sql              string
	// geometryFormat selects how the geometry column is decoded:
	// "" / "gpkg" (default; GeoPackage binary header + WKB), "wkb" (plain
	// WKB without the GeoPackage header), "wkt" (WKT text) or "mos"
	// (opaque MapplGIS binary blob).
	geometryFormat string
	// mosConfig holds the resolved MOS quantization settings used with
	// geometry_format = "mos".
	mosConfig codec.MOSConfig
	// boundFieldnames holds the raw bounds columns (minx/maxx/miny/maxy,
	// in that order) detected on a non-GPKG format table at registration.
	// They act as a coarse SQL filter mirroring the MySQL provider's MOS
	// bounds filter; nil when the table does not carry them.
	boundFieldnames *[4]string
	// isMapplGIS records that the table satisfied the canonical MapplGIS
	// contract (DDL + PK + indexes + OKEY=1 blob) at registration. After
	// NewTileProvider this state is final; tile requests never re-detect.
	isMapplGIS bool
	// mapplSource distinguishes how the layer was detected as MapplGIS:
	// canonical table detection (SystemInfo guaranteed) or the SQL sample
	// probe (no SystemInfo; explicit CRS config required).
	mapplSource codec.MapplGISSource
	// mapplSysInfo is the parsed layer self-description; valid only when
	// mapplSource is MapplGISTableCanonical.
	mapplSysInfo mos.SystemInfo
	// bboxFields holds the resolved bounds field names (layer > provider >
	// defaults) used by the bounds-backed custom-SQL !BBOX! predicate and
	// excluded from feature tags. For tablename layers with detected bounds
	// columns boundFieldnames stays authoritative; bboxFields mirrors it so
	// tag exclusion and the predicate builder share one contract.
	bboxFields codec.BBoxFields
	// bboxTable qualifies bounds references in custom SQL, not result column names.
	bboxTable string
	// crsExplicit records whether srid/crs_defn was set explicitly at
	// provider or layer level, suppressing source-metadata CRS inference.
	crsExplicit bool
	// crsConfigured separates caller declaration from later SystemInfo inference.
	crsConfigured bool
	// tileQueryPlan caches the registration-time spatial pre-filter plan
	// for the tile query (UPSTREAM debt 0.6b): planRTree when the
	// rtree_<table>_<geom> shadow table exists (checked via sqlite_master
	// once per layer at registration, never per request), planBBox when the
	// table carries bounds columns, planScan otherwise.
	tileQueryPlan        tileQueryPlan
	temporalMapping      provider.TemporalMapping
	temporalScale        int64
	featureQueryError    error
	featureColumns       []string
	featureRowIDAlias    string
	spatialMetadata      provider.SpatialMetadata
	spatialError         error
	nativeZ, nativeM     int
	heightProjection     *crsconfig.HeightProjection
	featureCRS           provider.FeatureCRSDefinition
	featureCRSProjection *crsconfig.FeatureProjection
	featureCRSError      error
	dimensionalSample    bool
	filterProfile        *featureFilterProfile
	filterError          error
}

type featureFilterColumn struct {
	name, declaration        string
	notNull, primary, hidden int
	kind                     provider.QueryableType
}

type featureFilterProfile struct {
	catalog             provider.FeatureQueryables
	columns             map[string]featureFilterColumn
	schemaSQL, encoding string
	schemaColumns       []featureFilterColumn
}

// TemporalMapping returns immutable registration-time temporal metadata.
func (l Layer) TemporalMapping() (provider.TemporalMapping, error) { return l.temporalMapping, nil }

// FeatureQuerySupported reports whether this layer has a proven query profile.
func (l Layer) FeatureQuerySupported() error { return errors.Join(l.featureQueryError, l.spatialError) }

// SpatialMetadata returns immutable registration-time dimensional evidence.
func (l Layer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return l.spatialMetadata, l.spatialError
}

// tileQueryPlan selects the spatial pre-filter baked into the tile-query
// SQL of a native (GeoPackage geometry) tablename layer. The plan is
// chosen and cached at layer registration (UPSTREAM debt 0.6b) so every
// tile request keeps exactly one SQL round trip and never probes per tile.
type tileQueryPlan int

const (
	// planRTree is the upstream plan: JOIN the rtree_<table>_<column>
	// shadow table and filter on its bounds via !BBOX!.
	planRTree tileQueryPlan = iota
	// planBBox filters on bounds columns of the table itself (the resolved
	// bbox_*_fieldname names or the legacy minx/maxx/miny/maxy) via !BBOX!.
	planBBox
	// planScan has no spatial pre-filter in SQL: the tile query scans the
	// table and the mandatory in-memory exact filter drops out-of-tile
	// features.
	planScan
)

func (l Layer) Name() string            { return l.name }
func (l Layer) GeomType() geom.Geometry { return l.geomType }
func (l Layer) SRID() uint64            { return l.srid }
func (l Layer) IDFieldName() string     { return l.idFieldname }
func (l Layer) GeomFieldName() string   { return l.geomFieldname }

// IsMapplGIS reports whether the layer was detected as MapplGIS at
// registration time (either table canonical or SQL sample).
func (l Layer) IsMapplGIS() bool { return l.isMapplGIS }

// MapplGISSource reports the detection source. SystemInfo is only guaranteed
// for MapplGISTableCanonical.
func (l Layer) MapplGISSource() codec.MapplGISSource { return l.mapplSource }
