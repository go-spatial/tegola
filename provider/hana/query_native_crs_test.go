package hana

import "testing"

func TestFeatureNativeCanonicalTuple(t *testing.T) {
	base := featureNativeColumn{SRID: 4326, SRS: featureNativeSRS{Definition: featureNativeWGS84Definition, Transform: "+proj=longlat +datum=WGS84 +no_defs", RoundEarth: "TRUE", Organization: "EPSG", OrganizationID: 4326}}
	if srid, ok := featureCanonicalNativeSRID(base); !ok || srid != 4326 {
		t.Fatal("canonical rejected")
	}
	planar := base
	planar.SRID = 1000004326
	planar.SRS.RoundEarth = "FALSE"
	if srid, ok := featureCanonicalNativeSRID(planar); !ok || srid != 4326 {
		t.Fatal("planar equivalent rejected")
	}
	for _, change := range []func(*featureNativeColumn){
		func(c *featureNativeColumn) { c.SRS.Definition += " " },
		func(c *featureNativeColumn) { c.SRS.Transform = "+proj=longlat +datum=NAD83 +no_defs" },
		func(c *featureNativeColumn) { c.SRS.RoundEarth = "FALSE" },
		func(c *featureNativeColumn) { c.SRS.Organization = "other" },
		func(c *featureNativeColumn) { c.SRS.OrganizationID = 3857 },
		func(c *featureNativeColumn) { c.SRID = 3857 },
		func(c *featureNativeColumn) { c.SRID = 1000003857 },
	} {
		c := base
		change(&c)
		if _, ok := featureCanonicalNativeSRID(c); ok {
			t.Fatalf("changed definition admitted %+v", c)
		}
	}
	a := featureCatalog{Native: []featureNativeColumn{base}}
	b := featureCatalog{Native: []featureNativeColumn{base}}
	b.Native[0].SRS.Transform += " "
	if a.equal(b) {
		t.Fatal("same-SRID definition drift hidden")
	}
}
