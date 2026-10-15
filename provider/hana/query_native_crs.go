package hana

// These complete immutable tuples were verified against HANA 2.00.088 and the
// canonical longitude/latitude WGS84 adapter. A nominal EPSG label is not proof.
// Other database definitions remain unsupported, even with the same SRS ID.
type featureNativeSRS struct {
	Definition, Transform, RoundEarth, Organization string
	OrganizationID                                  int64
}

const featureNativeWGS84Definition = `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`

func featureCanonicalNativeSRID(native featureNativeColumn) (uint64, bool) {
	expected := featureNativeSRS{
		Definition: featureNativeWGS84Definition,
		Transform:  "+proj=longlat +datum=WGS84 +no_defs",
		RoundEarth: "TRUE", Organization: "EPSG", OrganizationID: 4326,
	}
	switch native.SRID {
	case 4326:
	case 1000004326:
		expected.RoundEarth = "FALSE"
	default:
		return 0, false
	}
	return 4326, native.SRS == expected
}
