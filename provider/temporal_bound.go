package provider

import "time"

// IsPositiveLeapSecondPredecessor reports whether t's ordinary UTC second
// precedes a verified inserted second. Fractions and timezone presentation do
// not change the insertion. Dates derive from IERS Leap_Second.dat, retrieved
// 2026-10-01, Bulletin C72, SHA256 6cb6f5d4b819f2e568e25db4b0b26d89
// dedf031fdffb18bc94d40f4e94e268d7. The 1972 baseline is not an insertion.
func IsPositiveLeapSecondPredecessor(t time.Time) bool {
	u := t.UTC()
	if u.Hour() != 23 || u.Minute() != 59 || u.Second() != 59 {
		return false
	}
	switch u.Format("2006-01-02") {
	case "1972-06-30", "1972-12-31", "1973-12-31", "1974-12-31", "1975-12-31",
		"1976-12-31", "1977-12-31", "1978-12-31", "1979-12-31", "1981-06-30",
		"1982-06-30", "1983-06-30", "1985-06-30", "1987-12-31", "1989-12-31",
		"1990-12-31", "1992-06-30", "1993-06-30", "1994-06-30", "1995-12-31",
		"1997-06-30", "1998-12-31", "2005-12-31", "2008-12-31", "2012-06-30",
		"2015-06-30", "2016-12-31":
		return true
	}
	return false
}

func validateTemporalEndpoint(t *time.Time, fraction string, leap bool) error {
	if t == nil && (fraction != "" || leap) {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "fraction and leap marker require an endpoint"}
	}
	for i := 0; i < len(fraction); i++ {
		if fraction[i] < '0' || fraction[i] > '9' {
			return InvalidFeatureQueryError{Field: "temporal", Reason: "subnanosecond fraction must contain decimal digits"}
		}
	}
	if leap && !IsPositiveLeapSecondPredecessor(*t) {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "leap marker does not identify an announced positive insertion"}
	}
	return nil
}

func compareTemporalEndpoints(a time.Time, af string, al bool, b time.Time, bf string, bl bool) int {
	if a.Unix() < b.Unix() {
		return -1
	}
	if a.Unix() > b.Unix() {
		return 1
	}
	if al != bl {
		if al {
			return 1
		}
		return -1
	}
	if a.Nanosecond() < b.Nanosecond() {
		return -1
	}
	if a.Nanosecond() > b.Nanosecond() {
		return 1
	}
	for i := 0; i < len(af) || i < len(bf); i++ {
		x, y := byte('0'), byte('0')
		if i < len(af) {
			x = af[i]
		}
		if i < len(bf) {
			y = bf[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}
