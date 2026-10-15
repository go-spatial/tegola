package provider

// CRS84h identifies longitude, latitude and WGS84 ellipsoidal height in metres.
const CRS84h = "http://www.opengis.net/def/crs/OGC/0/CRS84h"

// Extent3D contains minX, minY, minZ, maxX, maxY, maxZ in that order.
type Extent3D [6]float64

// CoordinateDimension describes source coordinates; M is never height.
type CoordinateDimension uint8

const (
	DimensionUnknown CoordinateDimension = iota
	DimensionXY
	DimensionXYZ
	DimensionMixedXYXYZ
)

// SpatialMetadata is an immutable source declaration, not sampled row evidence.
type SpatialMetadata struct {
	Dimension   CoordinateDimension
	VerticalCRS string
}

// Validate checks the initial dimensional and vertical reference profile.
func (m SpatialMetadata) Validate() error {
	switch m.Dimension {
	case DimensionXY:
		if m.VerticalCRS != "" {
			return InvalidFeatureQueryError{Field: "vertical_crs", Reason: "XY source cannot declare a height reference"}
		}
	case DimensionXYZ, DimensionMixedXYXYZ:
		if m.VerticalCRS != CRS84h {
			return InvalidFeatureQueryError{Field: "vertical_crs", Reason: "XYZ source requires CRS84h ellipsoidal metres"}
		}
	default:
		return InvalidFeatureQueryError{Field: "spatial_dimension", Reason: "source dimension is unknown or unsupported"}
	}
	return nil
}

// SpatialLayerInfo supplies copied metadata without changing legacy LayerInfo.
type SpatialLayerInfo interface {
	SpatialMetadata() (SpatialMetadata, error)
}

// FeatureSourceLayerInfo separates immutable raw feature coordinates from the
// legacy tile presentation SRID. Zero is unknown and must not fall back to tiles.
type FeatureSourceLayerInfo interface {
	FeatureSourceSRID() uint64
}
