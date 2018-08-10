package style

const (
	SourceTypeVector  = "vector"
	SourceTypeRaster  = "raster"
	SourceTypeGeoJSON = "geojson"
	SourceTypeImage   = "image"
	SourceTypeVideo   = "video"
	SourceTypeCanvas  = "canvas"
)

type Source struct {
	Type string `json:"type"`
	// An array of one or more tile source URLs, as in the TileJSON spec.
	Tiles []string `json:"tiles,omitempty"`
	// defaults to 0 if not set
	MinZoom int `json:"minzoom,omitempty"`
	// defaults to 22 if not set
	MaxZoom int `json:"maxzoom,omitempty"`
	// url to TileJSON resource
	URL string `json:"url,omitempty"`
}
