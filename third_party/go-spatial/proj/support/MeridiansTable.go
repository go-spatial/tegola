// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

// MeridiansTableEntry holds a constant prime meridian item
type MeridiansTableEntry struct {
	ID         string
	Definition string
}

// MeridiansTable holds all the globally known (prime) meridians
var MeridiansTable = map[string]MeridiansTableEntry{
	"greenwich":  {"greenwich", "0dE"},
	"lisbon":     {"lisbon", "9d07'54.862\"W"},
	"paris":      {"paris", "2d20'14.025\"E"},
	"bogota":     {"bogota", "74d04'51.3\"W"},
	"madrid":     {"madrid", "3d41'16.58\"W"},
	"rome":       {"rome", "12d27'8.4\"E"},
	"bern":       {"bern", "7d26'22.5\"E"},
	"jakarta":    {"jakarta", "106d48'27.79\"E"},
	"ferro":      {"ferro", "17d40'W"},
	"brussels":   {"brussels", "4d22'4.71\"E"},
	"stockholm":  {"stockholm", "18d3'29.8\"E"},
	"athens":     {"athens", "23d42'58.815\"E"},
	"oslo":       {"oslo", "10d43'22.5\"E"},
	"copenhagen": {"copenhagen", "12d34'40.35\"E"},
}
