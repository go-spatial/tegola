// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

func init() {

	for _, entry := range DatumsTable {
		pl, err := NewProjString(entry.DefinitionString)
		if err != nil {
			panic(err)
		}

		entry.Definition = pl
	}
}

//---------------------------------------------------------------------

// DatumTableEntry holds a constant datum item
type DatumTableEntry struct {
	ID               string
	DefinitionString string
	EllipseID        string // note this is what the global table will key off of, not ID
	Comments         string
	Definition       *ProjString
}

// DatumsTable is the global list of datum constants
//
// TODO: the EllipseIDs for Greek_Geodetic_Reference_System_1987 and North_American_Datum_1983
// are the same, so I'm putting the Greek one under the key "GGRS87"
// TODO: "Potsdam Rauenberg 1950 DHDN" and "Hermannskogel" have the same problem, so we'll use the ID for Hermannskogel
var DatumsTable = map[string]*DatumTableEntry{
	"WGS84":         {"WGS84", "towgs84=0,0,0", "WGS84", "", nil},
	"GGRS87":        {"GGRS87", "towgs84=-199.87,74.79,246.62", "GRS80", "Greek_Geodetic_Reference_System_1987", nil},
	"GRS80":         {"NAD83", "towgs84=0,0,0", "GRS80", "North_American_Datum_1983", nil},
	"clrk66":        {"NAD27", "nadgrids=@conus,@alaska,@ntv2_0.gsb,@ntv1_can.dat", "clrk66", "North_American_Datum_1927", nil},
	"bessel":        {"potsdam" /*"towgs84=598.1,73.7,418.2,0.202,0.045,-2.455,6.7",*/, "nadgrids=@BETA2007.gsb", "bessel", "Potsdam Rauenberg 1950 DHDN", nil},
	"clrk80ign":     {"carthage", "towgs84=-263.0,6.0,431.0", "clrk80ign", "Carthage 1934 Tunisia", nil},
	"hermannskogel": {"hermannskogel", "towgs84=577.326,90.129,463.919,5.137,1.474,5.297,2.4232", "bessel", "Hermannskogel", nil},
	"mod_airy":      {"ire65", "towgs84=482.530,-130.596,564.557,-1.042,-0.214,-0.631,8.15", "mod_airy", "Ireland 1965", nil},
	"intl":          {"nzgd49", "towgs84=59.47,-5.04,187.44,0.47,-0.1,1.024,-4.5993", "intl", "New Zealand Geodetic Datum 1949", nil},
	"airy":          {"OSGB36", "towgs84=446.448,-125.157,542.060,0.1502,0.2470,0.8421,-20.4894", "airy", "Airy 1830", nil},
}
