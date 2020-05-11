// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import (
	"regexp"
	"strconv"

	"github.com/go-spatial/proj/merror"

	"github.com/go-spatial/proj/mlog"
)

// DMSToDD converts a degrees-minutes-seconds string to decimal-degrees
//
// Using an 8-part regexp, we support this format:
//    [+-] nnn [°Dd] nnn ['Mm] nnn.nnn ["Ss] [NnEeWwSs]
//
// TODO: the original dmstor() may support more, but the parsing code
// is messy and we don't have any testcases at this time.
func DMSToDD(input string) (float64, error) {

	mlog.Debugf("%s", input)

	deg := `\s*(-|\+)?\s*(\d+)\s*([°Dd]?)` // t1, t2, t3
	min := `\s*(\d+)?\s*(['Mm]?)`          // t4, t5
	sec := `\s*(\d+\.?\d*)?\s*(["Ss])?`    // t6, t7
	news := `\s*([NnEeWwSs]?)`             // t8
	expr := "^" + deg + min + sec + news + "$"
	r := regexp.MustCompile(expr)

	tokens := r.FindStringSubmatch(input)
	if tokens == nil {
		return 0.0, merror.New(merror.InvalidArg)
	}

	sign := tokens[1]
	d := tokens[2]
	m := tokens[4]
	s := tokens[6]
	dir := tokens[8]

	var df, mf, sf float64
	var err error

	df, err = strconv.ParseFloat(d, 64)
	if err != nil {
		return 0.0, err
	}
	if m != "" {
		mf, err = strconv.ParseFloat(m, 64)
		if err != nil {
			return 0.0, err
		}
	}
	if s != "" {
		sf, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return 0.0, err
		}
	}

	dd := df + mf/60.0 + sf/3600.0
	if sign == "-" {
		dd = -dd
	}

	if dir == "S" || dir == "s" || dir == "W" || dir == "w" {
		dd = -dd
	}

	return dd, nil
}

// DMSToR converts a DMS string to radians
func DMSToR(input string) (float64, error) {

	dd, err := DMSToDD(input)
	if err != nil {
		return 0.0, err
	}

	r := DDToR(dd)

	return r, nil
}

// DDToR converts decimal degrees to radians
func DDToR(deg float64) float64 {
	const degToRad = 0.017453292519943296
	r := deg * degToRad
	return r
}

// RToDD converts radians to decimal degrees
func RToDD(r float64) float64 {
	const radToDeg = 1.0 / 0.017453292519943296
	deg := r * radToDeg
	return deg
}

// ConvertArcsecondsToRadians converts from arc secs to rads
func ConvertArcsecondsToRadians(s float64) float64 {
	// Pi/180/3600
	r := s * 4.84813681109535993589914102357e-6
	return r
}
