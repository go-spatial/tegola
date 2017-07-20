package style

const (
	LayerTypeFill          = "fill"
	LayerTypeLine          = "line"
	LayerTypeSymbol        = "symbol"
	LayerTypeCircle        = "circle"
	LayerTypeFillExtrusion = "fill-extrusion"
	LayerTypeRaster        = "raster"
	LayerTypeBackgroudn    = "background"
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
	LineColor string `json:"line-color,omitempty"`
	FillColor string `json:"fill-color,omitempty"`
}

const (
	LayoutVisible     = "visible"
	LayoutVisibleNone = "none"
)

type LayerLayout struct {
	Visibility string `json:"visibility,omitempty"`
}
