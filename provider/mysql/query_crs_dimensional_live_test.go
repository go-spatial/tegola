package mysql

import (
	"context"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestFeatureLiveCRSDefinitionDimensional(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, connection := featureLiveDatabase(t, flavor)
			for _, dimension := range []string{"xyz", "mixed_xy_xyz"} {
				for _, custom := range []bool{false, true} {
					name := dimension + "/ordinary"
					if custom {
						name = dimension + "/custom"
					}
					t.Run(name, func(t *testing.T) {
						table := featureLiveTable(t, db, "source_id BIGINT UNSIGNED UNIQUE, source_geom TEXT, selected INTEGER")
						rows := [][]any{{1, "POINT Z(15 30 70)", 1}, {2, "POINT Z(15 30 7)", 1}, {3, "POINT Z(15 30 8)", 1}, {4, nil, 1}, {90, "POINT Z(15 30 7)", 0}}
						if dimension == "mixed_xy_xyz" {
							rows[2][1] = "POINT(15 30)"
						}
						for _, row := range rows {
							if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" (source_id,source_geom,selected) VALUES (?,?,?)", row...); err != nil {
								t.Fatal(err)
							}
						}
						layer := map[string]any{"name": "items", "tablename": table, "id_fieldname": "source_id", "geometry_fieldname": "source_geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "spatial_dimension": dimension, "vertical_crs": provider.CRS84h, "fields": []string{"source_id"}}
						if custom {
							layer["feature_sql"] = "SELECT s.source_id AS id, s.source_geom AS geom FROM " + featureQuoteIdentifier(table) + " AS s WHERE s.selected = 1"
							layer["id_fieldname"], layer["geometry_fieldname"] = "id", "geom"
						}
						p := featureLiveProvider(t, connection, layer)
						if _, err := p.layers["items"].FeatureCRSDefinition(); err != nil {
							t.Fatal(err)
						}
						definition, _ := crsconfig.CanonicalFeatureDefinition(3857)
						q := provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, BoundsCRSDefinition: definition, BoundsVerticalCRS: provider.CRS84h, Bounds3D: []provider.Extent3D{{1669792, 3503549, 6, 1669793, 3503550, 9}}}
						want := []uint64{2, 3, 4}
						if !custom {
							want = append(want, 90)
						}
						var ids []uint64
						checkGeometry := func(f *provider.Feature) {
							var expected geom.Geometry
							switch f.ID {
							case 2, 90:
								expected = geom.PointZ{15, 30, 7}
							case 3:
								expected = geom.PointZ{15, 30, 8}
								if dimension == "mixed_xy_xyz" {
									expected = geom.Point{15, 30}
								}
							case 4:
							default:
								t.Errorf("unexpected ID %d", f.ID)
							}
							if !reflect.DeepEqual(f.Geometry, expected) {
								t.Errorf("ID %d geometry got %#v want %#v", f.ID, f.Geometry, expected)
							}
						}
						result, err := p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { checkGeometry(f); ids = append(ids, f.ID); return nil })
						if err != nil || !reflect.DeepEqual(ids, want) || result.NumberMatched == nil || *result.NumberMatched != uint64(len(want)) {
							t.Fatal(ids, result, err)
						}
						q.Limit, q.Offset = 1, 1
						ids = nil
						result, err = p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { checkGeometry(f); ids = append(ids, f.ID); return nil })
						if err != nil || !reflect.DeepEqual(ids, []uint64{3}) || !result.HasMore || result.NumberReturned != 1 || (result.NumberMatched != nil && *result.NumberMatched != uint64(len(want))) {
							t.Fatal(ids, result, err)
						}
					})
				}
			}
		})
	}
}
