package gpkg

import "testing"

var (
	GPKGAthensFilePath       = "test_data/athens-osm-20170921.gpkg"
	GPKGNaturalEarthFilePath = "test_data/natural_earth_minimal.gpkg"
	GPKGPuertoMontFilePath   = "test_data/puerto_mont-osm-20170922.gpkg"
)

func TestCleanup(t *testing.T) {
	type tcase struct {
		config map[string]interface{}
	}

	fn := func(t *testing.T, tc tcase) {
		_, err := NewTileProvider(tc.config)
		if err != nil {
			t.Fatalf("err creating NewTileProvider: %v", err)
			return
		}

		if len(providers) != 1 {
			t.Errorf("expecting 1 providers, got %v", len(providers))
			return
		}

		Cleanup()

		if len(providers) != 0 {
			t.Errorf("expecting 0 providers, got %v", len(providers))
			return
		}
	}

	tests := map[string]tcase{
		"cleanup": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"amenity", "religion", "tourism", "shop"}},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"railway", "bridge", "tunnel"}},
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
