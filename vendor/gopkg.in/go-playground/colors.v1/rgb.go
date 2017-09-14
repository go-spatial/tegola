package colors

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	rgbString                    = "rgb(%d,%d,%d)"
	rgbCaptureRegexString        = "^rgb\\(\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])\\s*\\)$"
	rgbCaptureRegexPercentString = "^rgb\\(\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])%\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])%\\s*,\\s*(0|[1-9]\\d?|1\\d\\d?|2[0-4]\\d|25[0-5])%\\s*\\)$"
)

var (
	rgbCaptureRegex        = regexp.MustCompile(rgbCaptureRegexString)
	rgbCapturePercentRegex = regexp.MustCompile(rgbCaptureRegexPercentString)
)

// RGBColor represents an RGB color
type RGBColor struct {
	R uint8
	G uint8
	B uint8
}

// ParseRGB validates an parses the provided string into an RGBColor object
// supports both RGB 255 and RGB as percentages
func ParseRGB(s string) (*RGBColor, error) {

	s = strings.ToLower(s)

	var isPercent bool
	vals := rgbCaptureRegex.FindAllStringSubmatch(s, -1)

	if vals == nil || len(vals) == 0 || len(vals[0]) == 0 {

		vals = rgbCapturePercentRegex.FindAllStringSubmatch(s, -1)

		if vals == nil || len(vals) == 0 || len(vals[0]) == 0 {
			return nil, ErrBadColor
		}

		isPercent = true
	}

	r, _ := strconv.ParseUint(vals[0][1], 10, 8)
	g, _ := strconv.ParseUint(vals[0][2], 10, 8)
	b, _ := strconv.ParseUint(vals[0][3], 10, 8)

	if isPercent {
		r = uint64(math.Floor(float64(r)/100*255 + .5))
		g = uint64(math.Floor(float64(g)/100*255 + .5))
		b = uint64(math.Floor(float64(b)/100*255 + .5))
	}

	return &RGBColor{R: uint8(r), G: uint8(g), B: uint8(b)}, nil
}

// RGB validates and returns a new RGBColor object from the provided r, g, b values
func RGB(r, g, b uint8) (*RGBColor, error) {
	return &RGBColor{R: r, G: g, B: b}, nil
}

// String returns the string representation on the RGBColor
func (c *RGBColor) String() string {
	return fmt.Sprintf(rgbString, c.R, c.G, c.B)
}

// ToHEX converts the RGBColor to a HEXColor
func (c *RGBColor) ToHEX() *HEXColor {
	return &HEXColor{hex: fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)}
}

// ToRGB converts the RGBColor to an RGBColor
// it's here to satisfy the Color interface
func (c *RGBColor) ToRGB() *RGBColor {
	return c
}

// ToRGBA converts the RGBColor to an RGBAColor
func (c *RGBColor) ToRGBA() *RGBAColor {
	return &RGBAColor{R: c.R, G: c.G, B: c.B, A: 1}
}

// IsLight returns whether the color is perceived to be a light color
func (c *RGBColor) IsLight() bool {

	r := float64(c.R)
	g := float64(c.G)
	b := float64(c.B)

	hsp := math.Sqrt(0.299*math.Pow(r, 2) + 0.587*math.Pow(g, 2) + 0.114*math.Pow(b, 2))

	return hsp > 130
}

// IsDark returns whether the color is perceived to be a dark color
func (c *RGBColor) IsDark() bool {
	return !c.IsLight()
}
