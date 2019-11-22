// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import "strconv"

// ParseDate turns a date into a floating point year value.  Acceptable
// values are "yyyy.fraction" and "yyyy-mm-dd".  Anything else
// returns 0.0.
func ParseDate(s string) float64 {
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		year, err := strconv.Atoi(s[0:4])
		if err != nil {
			return 0.0
		}
		month, err := strconv.Atoi(s[5:7])
		if err != nil {
			return 0.0
		}
		day, err := strconv.Atoi(s[8:10])
		if err != nil {
			return 0.0
		}

		/* simplified calculation so we don't need to know all about months */
		return float64(year) + float64((month-1)*31+(day-1))/372.0
	}

	// TODO: handle the locale (as per pj_strtod)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		f = 0.0
	}
	return f
}
