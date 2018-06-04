package cmd

import (
	"strings"
	"fmt"
	"strconv"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths"
)

type Format struct {
	X, Y, Z uint
	Sep     string
}

var defaultTileNameFormat = Format{
	X:   0,
	Y:   1,
	Z:   2,
	Sep: "/",
}

// The variable should
type ErrTileNameFormat string

func (e ErrTileNameFormat) Error() string {
	return "invalid formatStr " + string(e)
}


func NewFormat(format string) (Format, error) {
	// if empty return default
	if format == "" {
		return defaultTileNameFormat, nil
	}

	// assert length of formatStr string is 4
	if len(format) != 4 {
		return defaultTileNameFormat, ErrTileNameFormat(format)
	}

	// check separator
	sep := format[0:1]
	if strings.ContainsAny(sep, "0123456789zxy") {
		return defaultTileNameFormat, ErrTileNameFormat(format)
	}

	zxy := format[1:4]

	ix := strings.Index(zxy, "x")
	iy := strings.Index(zxy, "y")
	iz := strings.Index(zxy, "z")

	// 0 + 1 + 2 = 3
	// strings.Index returns -1 if the substring is not in the string
	if ix + iy + iz != 3 {
		return defaultTileNameFormat, ErrTileNameFormat(format)
	}

	return Format{uint(ix), uint(iy), uint(iz), sep}, nil
}

func (f Format) String() string {
	var v [3]string
	v[f.X], v[f.Y], v[f.Z] = "x", "y", "z"
	return fmt.Sprintf("%[2]s%[1]s%[3]s%[1]s%[4]s", f.Sep, v[0], v[1], v[2])
}

func (f Format) Parse(val string) (z, x, y uint, err error) {
	parts := strings.Split(val, f.Sep)
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid zxy value (%v). expecting the formatStr %v", val, f)
	}

	zi, err := strconv.ParseUint(parts[f.Z], 10, 64)
	if err != nil || zi > tegola.MaxZ {
		return 0, 0, 0, fmt.Errorf("invalid Z value (%v)", zi)
	}

	maxXYatZ := maths.Exp2(zi) - 1

	xi, err := strconv.ParseUint(parts[f.X], 10, 64)
	if err != nil || xi > maxXYatZ {
		return 0, 0, 0, fmt.Errorf("invalid X value (%v)", xi)
	}

	yi, err := strconv.ParseUint(parts[f.Y], 10, 64)
	if err != nil || yi > maxXYatZ {
		return 0, 0, 0, fmt.Errorf("invalid Y value (%v)", yi)
	}

	return uint(zi), uint(xi), uint(yi), nil
}
