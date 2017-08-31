package style

const (
	LayerTypeFill          = "fill"
	LayerTypeLine          = "line"
	LayerTypeSymbol        = "symbol"
	LayerTypeCircle        = "circle"
	LayerTypeFillExtrusion = "fill-extrusion"
	LayerTypeRaster        = "raster"
	LayerTypeBackground    = "background"
)

type Layer struct {
	ID          string       `json:"id"`
	Source      string       `json:"source,omitempty"`
	SourceLayer string       `json:"source-layer,omitempty"`
	Type        string       `json:"type,omitempty"`
	Layout      *LayerLayout `json:"layout"`
	Paint       *LayerPaint  `json:"paint"`
}

type LayerPaint struct {
	LineColor        string `json:"line-color,omitempty"`
	FillColor        string `json:"fill-color,omitempty"`
	FillOutlineColor string `json:"fill-outline-color,omitempty"`
	FillOpacity      uint8  `json:"fill-opacity,omitempty"`
}

const (
	LayoutVisible     = "visible"
	LayoutVisibleNone = "none"
)

type LayerLayout struct {
	Visibility string `json:"visibility,omitempty"`
}
