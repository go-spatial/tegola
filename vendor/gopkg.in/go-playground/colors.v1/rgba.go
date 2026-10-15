package colors

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	rgbaString                    = "rgba(%d,%d,%d,%g)"
	rgbaCaptureRegexString        = "^rgba\\(\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])\\s*,\\s*(0\\.[0-9]*|[01])\\s*\\)$"
	rgbaCaptureRegexPercentString = "^rgba\\(\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])%\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])%\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])%\\s*,\\s*(0\\.[0-9]*|[01])\\s*\\)$"
)

var (
	rgbaCaptureRegex        = regexp.MustCompile(rgbaCaptureRegexString)
	rgbaCapturePercentRegex = regexp.MustCompile(rgbaCaptureRegexPercentString)
)

// RGBAColor represents an RGBA color
type RGBAColor struct {
	R uint8
	G uint8
	B uint8
	A float64
}

// ParseRGBA validates an parses the provided string into an RGBAColor object
// supports both RGBA 255 and RGBA as percentages
func ParseRGBA(s string) (*RGBAColor, error) {

	s = strings.ToLower(s)

	var isPercent bool

	vals := rgbaCaptureRegex.FindAllStringSubmatch(s, -1)

	if vals == nil || len(vals) == 0 || len(vals[0]) == 0 {

		vals = rgbaCapturePercentRegex.FindAllStringSubmatch(s, -1)

		if vals == nil || len(vals) == 0 || len(vals[0]) == 0 {
			return nil, ErrBadColor
		}

		isPercent = true
	}

	r, _ := strconv.ParseUint(vals[0][1], 10, 8)
	g, _ := strconv.ParseUint(vals[0][2], 10, 8)
	b, _ := strconv.ParseUint(vals[0][3], 10, 8)
	a, _ := strconv.ParseFloat(vals[0][4], 64)

	if isPercent {
		r = uint64(math.Floor(float64(r)/100*255 + .5))
		g = uint64(math.Floor(float64(g)/100*255 + .5))
		b = uint64(math.Floor(float64(b)/100*255 + .5))
	}

	return &RGBAColor{R: uint8(r), G: uint8(g), B: uint8(b), A: a}, nil
}

// RGBA validates and returns a new RGBAColor object from the provided r, g, b, a values
func RGBA(r, g, b uint8, a float64) (*RGBAColor, error) {

	if a < 0 || a > 1 {
		return nil, ErrBadColor
	}

	return &RGBAColor{R: r, G: g, B: b, A: a}, nil
}

func FromStdColor(c color.Color) *RGBAColor {
	r, g, b, a := c.RGBA()
	return &RGBAColor{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: float64(a>>8) / 0xff,
	}
}

// String returns the string representation on the RGBAColor
func (c *RGBAColor) String() string {
	return fmt.Sprintf(rgbaString, c.R, c.G, c.B, c.A)
}

// ToHEX converts the RGBAColor to a HEXColor
func (c *RGBAColor) ToHEX() *HEXColor {
	return &HEXColor{hex: fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)}
}

// ToRGB converts the RGBAColor to an RGBColor
func (c *RGBAColor) ToRGB() *RGBColor {
	return &RGBColor{R: c.R, G: c.G, B: c.B}
}

// ToRGBA converts the RGBAColor to an RGBAColor
// it's here to satisfy the Color interface
func (c *RGBAColor) ToRGBA() *RGBAColor {
	return c
}

// IsLight returns whether the color is perceived to be a light color
// NOTE: this is determined only by the RGB values, if you need to take
// the alpha into account see the IsLightAlpha function
func (c *RGBAColor) IsLight() bool {
	return c.ToRGB().IsLight()
}

// IsDark returns whether the color is perceived to be a dark color
// NOTE: this is determined only by the RGB values, if you need to take
// the alpha into account see the IsLightAlpha function
func (c *RGBAColor) IsDark() bool {
	return !c.IsLight()
}

// IsLightAlpha returns whether the color is perceived to be a light color
// based on RGBA values and the provided background color
// algorithm based of of post here: http://stackoverflow.com/a/12228643/3158232
func (c *RGBAColor) IsLightAlpha(bg Color) bool {

	// if alpha is 1 then RGB3 == RGB1
	if c.A == 1 {
		return c.IsLight()
	}

	// if alpha is 0 then RGB3 == RGB2
	if c.A == 0 {
		return bg.IsLight()
	}

	rgb2 := bg.ToRGB()

	r1 := float64(c.R)
	g1 := float64(c.G)
	b1 := float64(c.B)
	r2 := float64(rgb2.R)
	g2 := float64(rgb2.G)
	b2 := float64(rgb2.B)

	r3 := r2 + (r1-r2)*c.A
	g3 := g2 + (g1-g2)*c.A
	b3 := b2 + (b1-b2)*c.A

	rgb, _ := RGB(uint8(r3), uint8(g3), uint8(b3))

	return rgb.IsLight()
}

// IsDarkAlpha returns whether the color is perceived to be a dark color
// based on RGBA values and the provided background color
// algorithm based of of post here: http://stackoverflow.com/a/12228643/3158232
func (c *RGBAColor) IsDarkAlpha(bg Color) bool {
	return !c.IsLightAlpha(bg)
}
