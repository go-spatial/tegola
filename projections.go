package tegola

const (
	WebMercator = 3857
	WGS84       = 4326
)

var (
	WebMercatorBounds = [4]float64{-20026376.39, -20048966.10, 20026376.39, 20048966.10}
	WGS84Bounds       = [4]float64{-180.0, -85.0511, 180.0, 85.0511}
)
