package colors

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	hexRegexString = "^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"
	hexFormat      = "#%02x%02x%02x"
	hexShortFormat = "#%1x%1x%1x"
	hexToRGBFactor = 17
)

var (
	hexRegex = regexp.MustCompile(hexRegexString)
)

// HEXColor represents a HEX color
type HEXColor struct {
	hex string
}

// ParseHEX validates an parses the provided string into a HEXColor object
func ParseHEX(s string) (*HEXColor, error) {

	s = strings.ToLower(s)

	if !hexRegex.MatchString(s) {
		return nil, ErrBadColor
	}

	return &HEXColor{hex: s}, nil
}

// String returns the string representation on the HEXColor
func (c *HEXColor) String() string {
	return c.hex
}

// ToHEX converts the HEXColor to a HEXColor
// it's here to satisfy the Color interface
func (c *HEXColor) ToHEX() *HEXColor {
	return c
}

// ToRGB converts the HEXColor to and RGBColor
func (c *HEXColor) ToRGB() *RGBColor {

	var r, g, b uint8

	if len(c.hex) == 4 {
		fmt.Sscanf(c.hex, hexShortFormat, &r, &g, &b)
		r *= hexToRGBFactor
		g *= hexToRGBFactor
		b *= hexToRGBFactor
	} else {
		fmt.Sscanf(c.hex, hexFormat, &r, &g, &b)
	}

	return &RGBColor{R: r, G: g, B: b}
}

// ToRGBA converts the HEXColor to an RGBAColor
func (c *HEXColor) ToRGBA() *RGBAColor {

	rgb := c.ToRGB()

	return &RGBAColor{R: rgb.R, G: rgb.G, B: rgb.B, A: 1}
}

// IsLight returns whether the color is perceived to be a light color
func (c *HEXColor) IsLight() bool {
	return c.ToRGB().IsLight()
}

// IsDark returns whether the color is perceived to be a dark color
func (c *HEXColor) IsDark() bool {
	return !c.IsLight()
}
